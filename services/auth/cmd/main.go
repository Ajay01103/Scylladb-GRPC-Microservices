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

	"github.com/Ajay01103/go-notion/auth/config"
	"github.com/Ajay01103/go-notion/auth/db"
	"github.com/Ajay01103/go-notion/auth/gen/pb/pbconnect"
	"github.com/Ajay01103/go-notion/auth/internal/repository"
	"github.com/Ajay01103/go-notion/auth/internal/scyllastore"
	"github.com/Ajay01103/go-notion/auth/internal/service"
	"github.com/Ajay01103/go-notion/auth/internal/tokencache"
	"github.com/Ajay01103/go-notion/auth/server"
	"github.com/Ajay01103/go-notion/pkg/interceptor"
	pkglogger "github.com/Ajay01103/go-notion/pkg/logger"
	"github.com/Ajay01103/go-notion/pkg/token"
)

// jwksCacheEntry stores cached JWKS bytes with an expiry timestamp.
type jwksCacheEntry struct {
	Data      []byte
	ExpiresAt time.Time
}
// corsMiddleware allows Next.js or any other frontend to access Connect endpoints.
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
		fmt.Fprintf(os.Stderr, "auth service exited with error: %v\n", err)
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
	// The keyspace doesn't exist yet, so we can't include it in the cluster config
	baseCluster := db.NewCluster(db.ClusterConfig{
		Hosts:       cfg.ScyllaHosts,
		Port:        cfg.ScyllaPort,
		// No Keyspace here — it doesn't exist yet
		Username:    cfg.ScyllaUsername,
		Password:    cfg.ScyllaPassword,
		Consistency: gocql.LocalQuorum,
	})

	// 3. Wait for ScyllaDB to be reachable (60s timeout, 15 retries @ 2s = 30s max wait)
	logger.Info("waiting for scylladb to be ready", zap.Strings("hosts", cfg.ScyllaHosts))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	if err := db.WaitForReady(ctx, baseCluster, 15); err != nil {
		cancel()
		logger.Error("scylladb not ready", zap.Error(err))
		return fmt.Errorf("wait for scylladb: %w", err)
	}
	cancel()

	// 4. Bootstrap keyspace (create if it doesn't exist)
	logger.Info("bootstrapping scylladb keyspace")
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	if err := db.BootstrapKeyspace(ctx, baseCluster, cfg.ScyllaDatacenter, cfg.ScyllaReplicationFactor); err != nil {
		cancel()
		logger.Error("cannot bootstrap keyspace", zap.Error(err))
		return fmt.Errorf("bootstrap keyspace: %w", err)
	}
	cancel()

	// 5. NOW create the main session, keyspace is guaranteed to exist
	baseCluster.Keyspace = "auth_ks"
	session, err := db.NewSession(baseCluster)
	if err != nil {
		logger.Error("cannot create scylladb session", zap.Error(err))
		return fmt.Errorf("create session: %w", err)
	}
	defer session.Close()

	// 6. Run migrations (execute all idempotent .cql files)
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
	userRepo := repository.NewUserRepo(session)
	
	// Session-based stores (new model)
	sessionStore := scyllastore.NewSessionStore(session)
	revocationStore := scyllastore.NewSessionStore(session)
	
	// Shared auth cache for session state and revocation lookups
	authCache, err := tokencache.NewRistrettoCache()
	if err != nil {
		logger.Error("cannot create token cache", zap.Error(err))
		return fmt.Errorf("create token cache: %w", err)
	}
	
	eddsaKeyRetention := cfg.EDDSASigningKeyRetentionDuration
	if eddsaKeyRetention < cfg.RefreshTokenDuration {
		logger.Warn(
			"eddsa signing key retention is shorter than refresh token duration; clamping to refresh duration",
			zap.Duration("eddsaSigningKeyRetention", eddsaKeyRetention),
			zap.Duration("refreshTokenDuration", cfg.RefreshTokenDuration),
		)
		eddsaKeyRetention = cfg.RefreshTokenDuration
	}

	eddsaMaker, err := token.NewEDDSAMakerWithScylla(session, eddsaKeyRetention)
	if err != nil {
		logger.Error("cannot create EdDSA token maker", zap.Error(err))
		return fmt.Errorf("create eddsa token maker: %w", err)
	}
	var tokenMaker token.TokenMaker = eddsaMaker

	authService := service.New(userRepo, tokenMaker, sessionStore, revocationStore, authCache, cfg, logger)
	authServer := server.New(authService)

	loggingInterceptor := interceptor.NewLoggingInterceptor(logger)

	// 8. Start ConnectRPC server (HTTP/2 with h2c)
	mux := http.NewServeMux()

	// JWKS cache (ristretto) for faster JWKS retrieval
	jwksCache, err := tokencache.NewRistrettoCache()
	if err != nil {
		logger.Error("cannot create jwks cache", zap.Error(err))
		return fmt.Errorf("create jwks cache: %w", err)
	}

	// JWKS endpoint for token validation by other services
	mux.HandleFunc("GET /.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		// Try cache first
		if v, ok := jwksCache.Get("jwks:current"); ok {
			if entry, ok := v.(jwksCacheEntry); ok {
				if time.Now().Before(entry.ExpiresAt) {
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Cache-Control", "public, max-age=3600")
					w.Write(entry.Data)
					return
				}
			}
		}

		// Cache miss: try to load serialized JWKS from ScyllaDB table first
		var jwksText string
		if err := session.Query(`SELECT jwks_json FROM jwks_public_keys WHERE id = ?`, "current").WithContext(r.Context()).Scan(&jwksText); err == nil {
			jwksBytes := []byte(jwksText)
			jwksCache.Set("jwks:current", jwksCacheEntry{Data: jwksBytes, ExpiresAt: time.Now().Add(24 * time.Hour)}, 1)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "public, max-age=3600")
			w.Write(jwksBytes)
			return
		}

		// If not found in Scylla, export from EDDSAMaker (in-memory keys loaded at startup)
		jwksData, err := eddsaMaker.ExportPublicKeys()
		if err != nil {
			logger.Error("export jwks", zap.Error(err))
			http.Error(w, "Failed to export JWKS", http.StatusInternalServerError)
			return
		}

		// Store result in cache with 24h TTL (managed via ExpiresAt)
		jwksCache.Set("jwks:current", jwksCacheEntry{Data: jwksData, ExpiresAt: time.Now().Add(24 * time.Hour)}, 1)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Write(jwksData)
	})

	path, handler := pbconnect.NewAuthServiceHandler(
		authServer,
		connect.WithInterceptors(loggingInterceptor),
	)
	mux.Handle(path, corsMiddleware(handler))

	addr := fmt.Sprintf(":%s", cfg.GRPCPort)
	srv := &http.Server{
		Addr:    addr,
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}
	serverErrCh := make(chan error, 1)

	go func() {
		logger.Info("AUTH SERVICE started at ConnectRPC server", zap.String("addr", addr))
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
