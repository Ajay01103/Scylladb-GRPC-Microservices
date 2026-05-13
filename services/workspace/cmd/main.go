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
	pkglogger "github.com/Ajay01103/go-notion/pkg/logger"
	"github.com/Ajay01103/go-notion/pkg/token"
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

	// 2. Build a keyspace-less cluster config for bootstrap
	baseCluster := db.NewCluster(db.ClusterConfig{
		Hosts:       cfg.ScyllaHosts,
		Port:        cfg.ScyllaPort,
		Username:    cfg.ScyllaUsername,
		Password:    cfg.ScyllaPassword,
		Consistency: gocql.LocalQuorum,
	})

	// 3. Wait for ScyllaDB to be reachable
	logger.Info("waiting for scylladb to be ready", zap.Strings("hosts", cfg.ScyllaHosts))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	if err := db.WaitForReady(ctx, baseCluster, 15); err != nil {
		cancel()
		logger.Error("scylladb not ready", zap.Error(err))
		return fmt.Errorf("wait for scylladb: %w", err)
	}
	cancel()

	// 4. Bootstrap keyspace
	logger.Info("bootstrapping scylladb keyspace")
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	if err := db.BootstrapKeyspace(ctx, baseCluster, cfg.ScyllaDatacenter, cfg.ScyllaReplicationFactor); err != nil {
		cancel()
		logger.Error("cannot bootstrap keyspace", zap.Error(err))
		return fmt.Errorf("bootstrap keyspace: %w", err)
	}
	cancel()

	// 5. Create session with keyspace
	baseCluster.Keyspace = "workspace_ks"
	session, err := db.NewSession(baseCluster)
	if err != nil {
		logger.Error("cannot create scylladb session", zap.Error(err))
		return fmt.Errorf("create session: %w", err)
	}
	defer session.Close()

	// 6. Run migrations
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	hasMigrations, err := db.RunMigrations(ctx, session)
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

	// 7. Setup dependencies
	// Create remote validator for JWT verification via JWKS
	jwksValidator := token.NewRemoteValidator(cfg.JWKSEndpoint)

	// Create workspace service
	workspaceSvc := service.New(session, logger)

	// Create handlers
	workspaceServer := server.New(workspaceSvc)

	// Create interceptors
	loggingInterceptor := interceptor.NewLoggingInterceptor(logger)
	authInterceptor := interceptor.NewAuthInterceptor(jwksValidator)

	// 8. Start Connect RPC server
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
