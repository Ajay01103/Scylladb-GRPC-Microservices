package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Ajay01103/go-notion/whiteboard/gen/pb"
)

type BufferedOp struct {
	OpType  string
	Records []byte
	UserID  uuid.UUID
}

type Service struct {
	session *gocql.Session
	logger  *zap.Logger
}

func toGocqlUUID(id uuid.UUID) gocql.UUID {
	return gocql.UUID(id)
}

func fromGocqlUUID(id gocql.UUID) uuid.UUID {
	return uuid.UUID(id)
}

func New(session *gocql.Session, logger *zap.Logger) *Service {
	return &Service{session: session, logger: logger}
}

func (s *Service) CreateBoard(ctx context.Context, workspaceID, userID uuid.UUID, title string, isPrivate bool) (*pb.Board, error) {
	if ok, err := s.HasWorkspaceAccess(ctx, workspaceID, userID); err != nil || !ok {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("access denied")
	}

	boardID := uuid.New()
	now := time.Now().UTC()

	if err := s.session.Query(
		`INSERT INTO boards (workspace_id, board_id, title, created_by, is_private, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		toGocqlUUID(workspaceID), toGocqlUUID(boardID), title, toGocqlUUID(userID), isPrivate, now, now,
	).WithContext(ctx).Exec(); err != nil {
		return nil, fmt.Errorf("create board: %w", err)
	}

	if err := s.session.Query(
		`INSERT INTO boards_by_id (board_id, workspace_id, title, created_by, is_private, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		toGocqlUUID(boardID), toGocqlUUID(workspaceID), title, toGocqlUUID(userID), isPrivate, now, now,
	).WithContext(ctx).Exec(); err != nil {
		s.logger.Error("failed to insert boards_by_id", zap.Error(err), zap.String("board_id", boardID.String()))
	}

	return &pb.Board{
		Id:          boardID.String(),
		WorkspaceId: workspaceID.String(),
		Title:       title,
		CreatedBy:   userID.String(),
		IsPrivate:   isPrivate,
		CreatedAt:   timestamppb.New(now),
		UpdatedAt:   timestamppb.New(now),
	}, nil
}

func (s *Service) GetBoard(ctx context.Context, boardID uuid.UUID, requesterID uuid.UUID) (*pb.Board, error) {
	workspaceID, title, createdBy, isPrivate, createdAt, updatedAt, err := s.getBoardByID(ctx, boardID)
	if err != nil {
		return nil, err
	}

	if isPrivate && createdBy != requesterID {
		return nil, errors.New("access denied")
	}
	if ok, err := s.HasWorkspaceAccess(ctx, workspaceID, requesterID); err != nil || !ok {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("access denied")
	}

	return &pb.Board{
		Id:          boardID.String(),
		WorkspaceId: workspaceID.String(),
		Title:       title,
		CreatedBy:   createdBy.String(),
		IsPrivate:   isPrivate,
		CreatedAt:   timestamppb.New(createdAt),
		UpdatedAt:   timestamppb.New(updatedAt),
	}, nil
}

func (s *Service) ListWorkspaceBoards(ctx context.Context, workspaceID, requesterID uuid.UUID) ([]*pb.Board, error) {
	if ok, err := s.HasWorkspaceAccess(ctx, workspaceID, requesterID); err != nil || !ok {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("access denied")
	}

	iter := s.session.Query(
		`SELECT board_id, title, created_by, is_private, created_at, updated_at FROM boards WHERE workspace_id = ?`,
		toGocqlUUID(workspaceID),
	).WithContext(ctx).Iter()

	out := make([]*pb.Board, 0)
	var boardID gocql.UUID
	var title string
	var createdBy gocql.UUID
	var isPrivate bool
	var createdAt time.Time
	var updatedAt time.Time

	for iter.Scan(&boardID, &title, &createdBy, &isPrivate, &createdAt, &updatedAt) {
		createdByID := fromGocqlUUID(createdBy)
		if isPrivate && createdByID != requesterID {
			continue
		}
		out = append(out, &pb.Board{
			Id:          boardID.String(),
			WorkspaceId: workspaceID.String(),
			Title:       title,
			CreatedBy:   createdByID.String(),
			IsPrivate:   isPrivate,
			CreatedAt:   timestamppb.New(createdAt),
			UpdatedAt:   timestamppb.New(updatedAt),
		})
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("list boards: %w", err)
	}
	return out, nil
}

func (s *Service) AppendBoardOp(ctx context.Context, boardID, userID uuid.UUID, opType string, records []byte) error {
	if ok, err := s.CanAccessBoard(ctx, boardID, userID); err != nil || !ok {
		if err != nil {
			return err
		}
		return errors.New("access denied")
	}

	return s.session.Query(
		`INSERT INTO board_ops (board_id, op_id, op_type, records, user_id) VALUES (?, now(), ?, ?, ?)`,
		toGocqlUUID(boardID), opType, records, toGocqlUUID(userID),
	).WithContext(ctx).Exec()
}

func (s *Service) BatchAppendBoardOps(ctx context.Context, boardID uuid.UUID, ops []BufferedOp) error {
	batch := s.session.NewBatch(gocql.UnloggedBatch)
	for _, op := range ops {
		batch.Query(
			`INSERT INTO board_ops (board_id, op_id, op_type, records, user_id) VALUES (?, now(), ?, ?, ?)`,
			toGocqlUUID(boardID), op.OpType, op.Records, toGocqlUUID(op.UserID),
		)
	}
	return s.session.ExecuteBatch(batch.WithContext(ctx))
}

func (s *Service) UpsertBoardSnapshot(ctx context.Context, boardID uuid.UUID, document []byte) error {
	return s.session.Query(
		`INSERT INTO board_snapshots (board_id, snapshot_at, document, schema_version) VALUES (?, now(), ?, ?)`,
		toGocqlUUID(boardID), document, 1,
	).WithContext(ctx).Exec()
}

func (s *Service) LatestBoardSnapshot(ctx context.Context, boardID uuid.UUID) ([]byte, error) {
	var document []byte
	err := s.session.Query(
		`SELECT document FROM board_snapshots WHERE board_id = ? LIMIT 1`,
		toGocqlUUID(boardID),
	).WithContext(ctx).Scan(&document)
	if err == gocql.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("latest board snapshot: %w", err)
	}
	return document, nil
}

func (s *Service) CanAccessBoard(ctx context.Context, boardID, userID uuid.UUID) (bool, error) {
	workspaceID, _, createdBy, isPrivate, _, _, err := s.getBoardByID(ctx, boardID)
	if err != nil {
		return false, err
	}

	if isPrivate && createdBy != userID {
		return false, nil
	}

	return s.HasWorkspaceAccess(ctx, workspaceID, userID)
}

func (s *Service) HasWorkspaceAccess(ctx context.Context, workspaceID, userID uuid.UUID) (bool, error) {
	var role string
	err := s.session.Query(
		`SELECT role FROM workspace_ks.workspace_members_by_user WHERE user_id = ? AND workspace_id = ? LIMIT 1`,
		userID.String(), workspaceID.String(),
	).WithContext(ctx).Scan(&role)
	if err == gocql.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check workspace access: %w", err)
	}
	return role != "", nil
}

func (s *Service) getBoardByID(ctx context.Context, boardID uuid.UUID) (uuid.UUID, string, uuid.UUID, bool, time.Time, time.Time, error) {
	var workspaceID gocql.UUID
	var title string
	var createdBy gocql.UUID
	var isPrivate bool
	var createdAt time.Time
	var updatedAt time.Time

	err := s.session.Query(
		`SELECT workspace_id, title, created_by, is_private, created_at, updated_at FROM boards_by_id WHERE board_id = ?`,
		toGocqlUUID(boardID),
	).WithContext(ctx).Scan(&workspaceID, &title, &createdBy, &isPrivate, &createdAt, &updatedAt)
	if err == gocql.ErrNotFound {
		return uuid.Nil, "", uuid.Nil, false, time.Time{}, time.Time{}, errors.New("board not found")
	}
	if err != nil {
		return uuid.Nil, "", uuid.Nil, false, time.Time{}, time.Time{}, fmt.Errorf("get board: %w", err)
	}

	return fromGocqlUUID(workspaceID), title, fromGocqlUUID(createdBy), isPrivate, createdAt, updatedAt, nil
}
