package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	authpb "github.com/Ajay01103/go-notion/auth/gen/pb"
	authpbconnect "github.com/Ajay01103/go-notion/auth/gen/pb/pbconnect"
	"github.com/Ajay01103/go-notion/pkg/interceptor"
	"github.com/Ajay01103/go-notion/pkg/jwks"
	pkglogger "github.com/Ajay01103/go-notion/pkg/logger"
	"github.com/Ajay01103/go-notion/pkg/token"
	"github.com/Ajay01103/go-notion/whiteboard/config"
	"github.com/Ajay01103/go-notion/whiteboard/db"
	"github.com/Ajay01103/go-notion/whiteboard/gen/pb/pbconnect"
	"github.com/Ajay01103/go-notion/whiteboard/internal/realtime"
	"github.com/Ajay01103/go-notion/whiteboard/internal/service"
	"github.com/Ajay01103/go-notion/whiteboard/server"
)

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
		fmt.Fprintf(os.Stderr, "whiteboard service exited with error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	logger := pkglogger.New()
	defer logger.Sync()

	undo := zap.ReplaceGlobals(logger)
	defer undo()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

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
		return fmt.Errorf("connect to scylladb: %w", err)
	}
	defer session.Close()

	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	hasMigrations, err := db.Migrate(ctx, session)
	cancel()
	if err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	if hasMigrations {
		logger.Info("database migrations applied successfully")
	}

	jwksStore := jwks.NewScyllaStore(session)
	jwksCtx, jwksCancel := context.WithCancel(context.Background())
	defer jwksCancel()
	jwksCache, err := jwks.New(jwksCtx,
		jwks.WithJWKSURL(cfg.JWKSEndpoint),
		jwks.WithFetchTimeout(10*time.Second),
		jwks.WithRefreshInterval(1*time.Hour),
		jwks.WithMinRefreshInterval(15*time.Minute),
		jwks.WithScyllaDB(jwksStore, "whiteboard"),
	)
	if err != nil {
		return fmt.Errorf("create jwks cache: %w", err)
	}
	jwksVerifier := jwks.NewVerifier(jwksCache, token.TokenIssuer, token.TokenAudience)

	whiteboardSvc := service.New(session, logger)
	whiteboardServer := server.New(whiteboardSvc)
	whiteboardHub := realtime.NewHub(whiteboardSvc, logger)

	// Create a dedicated cancel hook for the hub, cancelled before server shutdown.
	_, hubCancel := context.WithCancel(context.Background())
	defer hubCancel()

	loggingInterceptor := interceptor.NewLoggingInterceptor(logger)
	authClient := authpbconnect.NewAuthServiceClient(http.DefaultClient, authRPCURL(), connect.WithGRPC())
	revocationChecker := interceptor.TokenRevocationCheckerFunc(func(ctx context.Context, rawToken string) error {
		response, err := authClient.ValidateToken(ctx, connect.NewRequest(&authpb.ValidateTokenRequest{AccessToken: rawToken}))
		if err != nil {
			return err
		}
		if !response.Msg.GetValid() {
			return errors.New("access token is revoked")
		}
		return nil
	})
	authInterceptor := interceptor.NewAuthInterceptorWithRevocation(jwksVerifier, revocationChecker)

	mux := http.NewServeMux()
	path, handler := pbconnect.NewWhiteboardServiceHandler(
		whiteboardServer,
		connect.WithInterceptors(authInterceptor, loggingInterceptor),
	)
	mux.Handle(path, corsMiddleware(handler))

	wsTicketHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := authenticateHTTP(r, jwksVerifier, revocationChecker)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ticket, err := whiteboardSvc.IssueWSTicket(r.Context(), userID)
		if err != nil {
			logger.Error("failed to issue websocket ticket", zap.Error(err), zap.String("user_id", userID.String()))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"ticket": ticket}); err != nil {
			logger.Error("failed to encode websocket ticket response", zap.Error(err))
		}
	})
	mux.Handle("POST /ws/ticket", corsMiddleware(wsTicketHandler))
	mux.Handle("OPTIONS /ws/ticket", corsMiddleware(wsTicketHandler))

	mux.HandleFunc("GET /ws/boards/{boardId}", func(w http.ResponseWriter, r *http.Request) {
		ticket := r.URL.Query().Get("ticket")
		userID, err := whiteboardSvc.RedeemWSTicket(r.Context(), ticket)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		whiteboardHub.HandleBoardWS(w, r, userID)
	})

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	addr := fmt.Sprintf(":%s", cfg.GRPCPort)
	srv := &http.Server{Addr: addr, Handler: h2c.NewHandler(mux, &http2.Server{})}
	serverErrCh := make(chan error, 1)
	go func() {
		logger.Info("WHITEBOARD SERVICE started at ConnectRPC server", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case err := <-serverErrCh:
		return fmt.Errorf("listen and serve: %w", err)
	case <-quit:
	}

	hubCancel()

	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	return nil
}

func authRPCURL() string {
	if value := os.Getenv("AUTH_RPC_URL"); value != "" {
		return value
	}
	return "http://localhost:50051"
}

func authenticateHTTP(r *http.Request, verifier *jwks.Verifier, revocationChecker interceptor.TokenRevocationChecker) (uuid.UUID, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return uuid.Nil, errors.New("authorization token is not provided")
	}

	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return uuid.Nil, errors.New("invalid authorization token format")
	}

	tokenStr := strings.TrimSpace(authHeader[len(bearerPrefix):])
	if tokenStr == "" {
		return uuid.Nil, errors.New("authorization token is empty")
	}

	claims, err := verifier.Verify(r.Context(), tokenStr)
	if err != nil {
		return uuid.Nil, err
	}
	if claims == nil || claims.Subject == "" {
		return uuid.Nil, errors.New("token subject is missing")
	}
	if err := revocationChecker.ValidateAccessToken(r.Context(), tokenStr); err != nil {
		return uuid.Nil, err
	}

	return uuid.Parse(claims.Subject)
}
