package interceptor

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/Ajay01103/go-notion/pkg/jwks"
	"github.com/google/uuid"
)

// contextKeyUserID is the context key for the authenticated user ID
type contextKey string

const ContextKeyUserID contextKey = "user_id"

type TokenVerifier interface {
	Verify(context.Context, string) (*jwks.Claims, error)
}

// NewAuthInterceptor creates a unary interceptor for JWT authentication.
// It validates bearer tokens via the provided verifier and injects the user ID into context.
// Unauthenticated requests are rejected before the handler is invoked.
func NewAuthInterceptor(validator TokenVerifier) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			// Extract bearer token from Authorization header
			authHeader := req.Header().Get("Authorization")
			if authHeader == "" {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authorization token is not provided"))
			}

			const bearerPrefix = "Bearer "
			if !strings.HasPrefix(authHeader, bearerPrefix) {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid authorization token format"))
			}

			tokenStr := authHeader[len(bearerPrefix):]
			if tokenStr == "" {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authorization token is empty"))
			}

			// Validate the access token via the shared jwks verifier.
			claims, err := validator.Verify(ctx, tokenStr)
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("token validation failed: %w", err))
			}
			if claims == nil || claims.Subject == "" {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("token subject is missing"))
			}

			// Inject the authenticated user ID into the request context
			newCtx := context.WithValue(ctx, ContextKeyUserID, claims.Subject)

			// Call the next handler with the enriched context
			return next(newCtx, req)
		}
	}
}

// UserIDFromContext extracts the authenticated user ID from the request context.
// Returns an error if the user ID is not present or invalid.
func UserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	val := ctx.Value(ContextKeyUserID)
	if val == nil {
		return uuid.UUID{}, errors.New("user id not found in context")
	}

	userIDStr, ok := val.(string)
	if !ok {
		return uuid.UUID{}, errors.New("user id is not a string")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("invalid user id format: %w", err)
	}

	return userID, nil
}
