package interceptor

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/Ajay01103/go-notion/pkg/token"
	"github.com/google/uuid"
)

// contextKeyUserID is the context key for the authenticated user ID
type contextKey string

const ContextKeyUserID contextKey = "user_id"

// NewAuthInterceptor creates a unary interceptor for JWT authentication.
// It validates bearer tokens via the provided RemoteValidator and injects the user ID into context.
// Unauthenticated requests are rejected before the handler is invoked.
func NewAuthInterceptor(validator *token.RemoteValidator) connect.UnaryInterceptorFunc {
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

			// Validate the access token via RemoteValidator (JWKS-based)
			payload, err := validator.VerifyAccessToken(tokenStr)
			if err != nil {
				switch err {
				case token.ErrExpiredToken:
					return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("access token expired"))
				case token.ErrInvalidToken:
					return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("access token is invalid"))
				default:
					return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("token validation failed: %w", err))
				}
			}

			// Inject the authenticated user ID into the request context
			newCtx := context.WithValue(ctx, ContextKeyUserID, payload.UserID.String())

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
