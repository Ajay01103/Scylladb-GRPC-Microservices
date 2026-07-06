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
	"sync"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	notesconfig "github.com/Ajay01103/go-notion/notes/config"
	"github.com/Ajay01103/go-notion/notes/db"
	"github.com/Ajay01103/go-notion/notes/gen/pb/pbconnect"
	"github.com/Ajay01103/go-notion/notes/internal/realtime"
	s3svc "github.com/Ajay01103/go-notion/notes/internal/s3"
	"github.com/Ajay01103/go-notion/notes/internal/service"
	"github.com/Ajay01103/go-notion/notes/server"
	"github.com/Ajay01103/go-notion/pkg/interceptor"
	"github.com/Ajay01103/go-notion/pkg/jwks"
	pkglogger "github.com/Ajay01103/go-notion/pkg/logger"
)

// ── Per-user ticket rate limiter ─────────────────────────────────────────────
// Allows at most wsTicketRateLimit requests per wsTicketRateWindow per userID.
// In-memory sliding-window: cheap, effective for a single instance. For a
// multi-instance deployment, swap this out for a Redis token bucket.

const (
	wsTicketRateLimit  = 10
	wsTicketRateWindow = time.Minute
)

type ticketRateLimiter struct {
	mu      sync.Mutex
	buckets map[uuid.UUID][]time.Time
}

func newTicketRateLimiter() *ticketRateLimiter {
	return &ticketRateLimiter{buckets: make(map[uuid.UUID][]time.Time)}
}

// allow returns true if the request should proceed, false if it is rate-limited.
func (r *ticketRateLimiter) allow(userID uuid.UUID) bool {
	now := time.Now()
	cutoff := now.Add(-wsTicketRateWindow)

	r.mu.Lock()
	defer r.mu.Unlock()

	// Evict timestamps outside the current window.
	ts := r.buckets[userID]
	keep := ts[:0]
	for _, t := range ts {
		if t.After(cutoff) {
			keep = append(keep, t)
		}
	}

	if len(keep) >= wsTicketRateLimit {
		r.buckets[userID] = keep
		return false
	}

	r.buckets[userID] = append(keep, now)
	return true
}

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
		fmt.Fprintf(os.Stderr, "notes service exited with error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	logger := pkglogger.New()
	defer logger.Sync()

	undo := zap.ReplaceGlobals(logger)
	defer undo()

	cfg, err := notesconfig.Load()
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
		jwks.WithScyllaDB(jwksStore, "notes"),
	)
	if err != nil {
		return fmt.Errorf("create jwks cache: %w", err)
	}
	jwksVerifier := jwks.NewVerifier(jwksCache, "")

	s3awscfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(cfg.S3Region),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...any) (aws.Endpoint, error) {
				return aws.Endpoint{URL: cfg.S3Endpoint, HostnameImmutable: true}, nil
			},
		)),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.S3AccessKeyID, cfg.S3SecretAccessKey, "")),
	)
	if err != nil {
		return fmt.Errorf("load s3 config: %w", err)
	}
	s3client := s3.NewFromConfig(s3awscfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
	s3Presigner := s3svc.New(s3client, cfg.S3Bucket)

	notesSvc := service.New(session, logger, s3Presigner)
	notesServer := server.New(notesSvc)
	notesHub := realtime.NewHub(notesSvc, logger)
	ticketLimiter := newTicketRateLimiter()

	loggingInterceptor := interceptor.NewLoggingInterceptor(logger)
	authInterceptor := interceptor.NewAuthInterceptor(jwksVerifier)

	mux := http.NewServeMux()

	path, handler := pbconnect.NewNotesServiceHandler(
		notesServer,
		connect.WithInterceptors(authInterceptor, loggingInterceptor),
	)
	mux.Handle(path, corsMiddleware(handler))

	// WS ticket exchange: browser fetches a short-lived ticket via REST then
	// passes it as a query param when upgrading, since WebSocket connections
	// cannot send custom Authorization headers.
	wsTicketHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := authenticateHTTP(r, jwksVerifier)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Bug-3 fix (availability): per-user sliding-window rate limit.
		// Prevents a valid-JWT holder from flooding ScyllaDB with Paxos writes.
		if !ticketLimiter.allow(userID) {
			logger.Warn("ws ticket rate limit exceeded", zap.String("user_id", userID.String()))
			w.Header().Set("Retry-After", "60")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		ticket, err := notesSvc.IssueWSTicket(r.Context(), userID)
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

	mux.HandleFunc("GET /ws/notes/{noteId}", func(w http.ResponseWriter, r *http.Request) {
		noteID, err := uuid.Parse(r.PathValue("noteId"))
		if err != nil {
			http.Error(w, "invalid note id", http.StatusBadRequest)
			return
		}

		ticket := r.URL.Query().Get("ticket")
		userID, err := notesSvc.RedeemWSTicket(r.Context(), ticket)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Bug-3 fix (security): verify the authenticated user can actually access
		// this note. Without this check any user with a valid ticket could connect
		// to an arbitrary noteId they don't own by supplying a foreign UUID in the
		// URL — the ticket only proves identity, not note-level authorisation.
		ok, err := notesSvc.CanAccessNote(r.Context(), noteID, userID)
		if err != nil {
			logger.Error("access check failed during ws upgrade",
				zap.Error(err),
				zap.String("note_id", noteID.String()),
				zap.String("user_id", userID.String()),
			)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		notesHub.HandleNoteWS(w, r, userID)
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
		logger.Info("NOTES SERVICE started at ConnectRPC server", zap.String("addr", addr))
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

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Bug-3 option C: flush all active rooms before the HTTP server closes.
	// This covers the window between SIGTERM and process exit where the
	// debounce timer may not have fired for the last edits in each room.
	notesHub.Shutdown(ctx)

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	return nil
}

func authenticateHTTP(r *http.Request, verifier *jwks.Verifier) (uuid.UUID, error) {
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

	return uuid.Parse(claims.Subject)
}
