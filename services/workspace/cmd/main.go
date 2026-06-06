package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"connectrpc.com/connect"
	"github.com/gocql/gocql"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/Ajay01103/go-notion/pkg/interceptor"
	"github.com/Ajay01103/go-notion/pkg/jwks"
	pkglogger "github.com/Ajay01103/go-notion/pkg/logger"
	"github.com/Ajay01103/go-notion/workspace/config"
	"github.com/Ajay01103/go-notion/workspace/db"
	"github.com/Ajay01103/go-notion/workspace/gen/pb/pbconnect"
	"github.com/Ajay01103/go-notion/workspace/internal/service"
	"github.com/Ajay01103/go-notion/workspace/server"
)

// corsMiddleware allows frontend to access Connect endpoints
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Connect-Protocol-Version, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "workspace service exited with error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	logger := pkglogger.New()
	defer logger.Sync()

	undo := zap.ReplaceGlobals(logger)
	defer undo()

	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		logger.Error("cannot load config", zap.Error(err))
		return fmt.Errorf("load config: %w", err)
	}

	// 2. Connect performs readiness checks, keyspace bootstrap, and session creation.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	session, err := db.Connect(ctx, db.Config{
		Hosts:             cfg.ScyllaHosts,
		Port:              cfg.ScyllaPort,
		Username:          cfg.ScyllaUsername,
		Password:          cfg.ScyllaPassword,
		Consistency:       gocql.LocalQuorum,
		Datacenter:        cfg.ScyllaDatacenter,
		ReplicationFactor: cfg.ScyllaReplicationFactor,
	})
	cancel()
	if err != nil {
		logger.Error("cannot connect to scylladb", zap.Error(err))
		return fmt.Errorf("connect to scylladb: %w", err)
	}
	defer session.Close()

	// 3. Run migrations
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	hasMigrations, err := db.Migrate(ctx, session)
	cancel()
	if err != nil {
		logger.Error("cannot run migrations", zap.Error(err))
		return fmt.Errorf("run migrations: %w", err)
	}
	if hasMigrations {
		logger.Info("database migrations applied successfully")
	} else {
		logger.Debug("database schema is up-to-date")
	}

	// schema sync removed: DB schema is expected to be managed externally / reset as needed

	// 4. Setup dependencies
	jwksStore := jwks.NewScyllaStore(session)
	jwksCtx, jwksCancel := context.WithCancel(context.Background())
	defer jwksCancel()
	jwksCache, err := jwks.New(jwksCtx,
		jwks.WithJWKSURL(cfg.JWKSEndpoint),
		jwks.WithFetchTimeout(10*time.Second),
		jwks.WithRefreshInterval(1*time.Hour),
		jwks.WithMinRefreshInterval(15*time.Minute),
		jwks.WithScyllaDB(jwksStore, "workspace"),
	)
	if err != nil {
		logger.Error("cannot create jwks cache", zap.Error(err))
		return fmt.Errorf("create jwks cache: %w", err)
	}

	jwksVerifier := jwks.NewVerifier(jwksCache, "")

	// Create workspace service
	workspaceSvc := service.New(session, logger)

	// Create handlers
	workspaceServer := server.New(workspaceSvc)

	// Create interceptors
	loggingInterceptor := interceptor.NewLoggingInterceptor(logger)
	authInterceptor := interceptor.NewAuthInterceptor(jwksVerifier)

	// 5. Start Connect RPC server
	mux := http.NewServeMux()

	path, handler := pbconnect.NewWorkspaceServiceHandler(
		workspaceServer,
		connect.WithInterceptors(authInterceptor, loggingInterceptor),
	)
	mux.Handle(path, corsMiddleware(handler))

	// Health check endpoint
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	addr := fmt.Sprintf(":%s", cfg.GRPCPort)
	srv := &http.Server{
		Addr:    addr,
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}
	serverErrCh := make(chan error, 1)

	go func() {
		logger.Info("WORKSPACE SERVICE started at ConnectRPC server", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case err := <-serverErrCh:
		logger.Error("server terminated unexpectedly", zap.Error(err))
		return fmt.Errorf("listen and serve: %w", err)
	case sig := <-quit:
		logger.Info("received shutdown signal", zap.String("signal", sig.String()))
	}

	logger.Info("shutting down server...")

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", zap.Error(err))
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	logger.Info("server stopped")

	return nil
}
