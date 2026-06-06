package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const wsTicketTTLSeconds = 120

func (s *Service) IssueWSTicket(ctx context.Context, userID uuid.UUID) (string, error) {
	ticket := uuid.NewString()
	now := time.Now().UTC()

	if err := s.session.Query(
		`INSERT INTO ws_tickets (ticket, user_id, created_at) VALUES (?, ?, ?) USING TTL `+fmt.Sprintf("%d", wsTicketTTLSeconds),
		ticket, toGocqlUUID(userID), now,
	).WithContext(ctx).Exec(); err != nil {
		return "", fmt.Errorf("issue ws ticket: %w", err)
	}

	s.logger.Debug("websocket ticket issued",
		zap.String("user_id", userID.String()),
		zap.Time("created_at", now),
		zap.Int("ttl_seconds", wsTicketTTLSeconds),
	)

	return ticket, nil
}

func (s *Service) RedeemWSTicket(ctx context.Context, ticket string) (uuid.UUID, error) {
	if ticket == "" {
		return uuid.Nil, errors.New("ticket is required")
	}

	var userID gocql.UUID
	err := s.session.Query(
		`SELECT user_id FROM ws_tickets WHERE ticket = ? LIMIT 1`,
		ticket,
	).WithContext(ctx).Scan(&userID)
	if err == gocql.ErrNotFound {
		return uuid.Nil, errors.New("ticket not found")
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("lookup ws ticket: %w", err)
	}

	if err := s.session.Query(
		`DELETE FROM ws_tickets WHERE ticket = ?`,
		ticket,
	).WithContext(ctx).Exec(); err != nil {
		return uuid.Nil, fmt.Errorf("delete ws ticket: %w", err)
	}

	s.logger.Debug("websocket ticket redeemed",
		zap.String("user_id", fromGocqlUUID(userID).String()),
	)

	return fromGocqlUUID(userID), nil
}
