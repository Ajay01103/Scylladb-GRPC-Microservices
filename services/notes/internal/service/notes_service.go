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

	pb "github.com/Ajay01103/go-notion/notes/gen/pb"
)

type BufferedUpdate struct {
	Data   []byte
	UserID uuid.UUID
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

func (s *Service) CreateNote(ctx context.Context, workspaceID, userID uuid.UUID, title string, isPrivate bool) (*pb.Note, error) {
	if ok, err := s.HasWorkspaceAccess(ctx, workspaceID, userID); err != nil || !ok {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("access denied")
	}

	noteID := uuid.New()
	now := time.Now().UTC()

	if err := s.session.Query(
		`INSERT INTO notes (workspace_id, note_id, title, created_by, is_private, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		toGocqlUUID(workspaceID), toGocqlUUID(noteID), title, toGocqlUUID(userID), isPrivate, now, now,
	).WithContext(ctx).Exec(); err != nil {
		return nil, fmt.Errorf("create note: %w", err)
	}

	if err := s.session.Query(
		`INSERT INTO notes_by_id (note_id, workspace_id, title, created_by, is_private, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		toGocqlUUID(noteID), toGocqlUUID(workspaceID), title, toGocqlUUID(userID), isPrivate, now, now,
	).WithContext(ctx).Exec(); err != nil {
		s.logger.Error("failed to insert notes_by_id", zap.Error(err), zap.String("note_id", noteID.String()))
	}

	return &pb.Note{
		Id:          noteID.String(),
		WorkspaceId: workspaceID.String(),
		Title:       title,
		CreatedBy:   userID.String(),
		IsPrivate:   isPrivate,
		CreatedAt:   timestamppb.New(now),
		UpdatedAt:   timestamppb.New(now),
	}, nil
}

func (s *Service) GetNote(ctx context.Context, noteID uuid.UUID, requesterID uuid.UUID) (*pb.Note, error) {
	workspaceID, title, createdBy, isPrivate, createdAt, updatedAt, err := s.getNoteByID(ctx, noteID)
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

	return &pb.Note{
		Id:          noteID.String(),
		WorkspaceId: workspaceID.String(),
		Title:       title,
		CreatedBy:   createdBy.String(),
		IsPrivate:   isPrivate,
		CreatedAt:   timestamppb.New(createdAt),
		UpdatedAt:   timestamppb.New(updatedAt),
	}, nil
}

func (s *Service) ListWorkspaceNotes(ctx context.Context, workspaceID, requesterID uuid.UUID) ([]*pb.Note, error) {
	if ok, err := s.HasWorkspaceAccess(ctx, workspaceID, requesterID); err != nil || !ok {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("access denied")
	}

	iter := s.session.Query(
		`SELECT note_id, title, created_by, is_private, created_at, updated_at FROM notes WHERE workspace_id = ?`,
		toGocqlUUID(workspaceID),
	).WithContext(ctx).Iter()

	out := make([]*pb.Note, 0)
	var noteID gocql.UUID
	var title string
	var createdBy gocql.UUID
	var isPrivate bool
	var createdAt time.Time
	var updatedAt time.Time

	for iter.Scan(&noteID, &title, &createdBy, &isPrivate, &createdAt, &updatedAt) {
		createdByID := fromGocqlUUID(createdBy)
		if isPrivate && createdByID != requesterID {
			continue
		}
		out = append(out, &pb.Note{
			Id:          noteID.String(),
			WorkspaceId: workspaceID.String(),
			Title:       title,
			CreatedBy:   createdByID.String(),
			IsPrivate:   isPrivate,
			CreatedAt:   timestamppb.New(createdAt),
			UpdatedAt:   timestamppb.New(updatedAt),
		})
	}

	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}

	return out, nil
}

func (s *Service) AppendNoteUpdate(ctx context.Context, noteID, userID uuid.UUID, data []byte) error {
	if ok, err := s.CanAccessNote(ctx, noteID, userID); err != nil || !ok {
		if err != nil {
			return err
		}
		return errors.New("access denied")
	}

	return s.session.Query(
		`INSERT INTO yjs_updates (note_id, update_id, data, user_id) VALUES (?, now(), ?, ?)`,
		toGocqlUUID(noteID), data, toGocqlUUID(userID),
	).WithContext(ctx).Exec()
}

func (s *Service) BatchAppendNoteUpdates(ctx context.Context, noteID uuid.UUID, updates []BufferedUpdate) error {
	batch := s.session.NewBatch(gocql.UnloggedBatch)
	for _, update := range updates {
		batch.Query(
			`INSERT INTO yjs_updates (note_id, update_id, data, user_id) VALUES (?, now(), ?, ?)`,
			toGocqlUUID(noteID), update.Data, toGocqlUUID(update.UserID),
		)
	}
	return s.session.ExecuteBatch(batch.WithContext(ctx))
}

func (s *Service) UpsertSnapshot(ctx context.Context, noteID uuid.UUID, stateVector []byte) error {
	return s.session.Query(
		`INSERT INTO yjs_snapshots (note_id, snapshot_id, state_vector) VALUES (?, now(), ?)`,
		toGocqlUUID(noteID), stateVector,
	).WithContext(ctx).Exec()
}

func (s *Service) LatestSnapshot(ctx context.Context, noteID uuid.UUID) ([]byte, uuid.UUID, error) {
	var snapshotID gocql.UUID
	var stateVector []byte
	err := s.session.Query(
		`SELECT snapshot_id, state_vector FROM yjs_snapshots WHERE note_id = ? LIMIT 1`,
		toGocqlUUID(noteID),
	).WithContext(ctx).Scan(&snapshotID, &stateVector)
	if err == gocql.ErrNotFound {
		return nil, uuid.Nil, nil
	}
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("latest snapshot: %w", err)
	}
	return stateVector, fromGocqlUUID(snapshotID), nil
}

func (s *Service) UpdatesSince(ctx context.Context, noteID uuid.UUID, after gocql.UUID) ([][]byte, error) {
	query := `SELECT data FROM yjs_updates WHERE note_id = ?`
	args := []any{toGocqlUUID(noteID)}
	if after != (gocql.UUID{}) {
		query = `SELECT data FROM yjs_updates WHERE note_id = ? AND update_id > ?`
		args = append(args, after)
	}
	iter := s.session.Query(query, args...).WithContext(ctx).Iter()
	updates := make([][]byte, 0)
	var data []byte
	for iter.Scan(&data) {
		copied := make([]byte, len(data))
		copy(copied, data)
		updates = append(updates, copied)
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("updates since: %w", err)
	}
	return updates, nil
}

func (s *Service) CanAccessNote(ctx context.Context, noteID, userID uuid.UUID) (bool, error) {
	workspaceID, _, createdBy, isPrivate, _, _, err := s.getNoteByID(ctx, noteID)
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

func (s *Service) getNoteByID(ctx context.Context, noteID uuid.UUID) (uuid.UUID, string, uuid.UUID, bool, time.Time, time.Time, error) {
	var workspaceID gocql.UUID
	var title string
	var createdBy gocql.UUID
	var isPrivate bool
	var createdAt time.Time
	var updatedAt time.Time

	err := s.session.Query(
		`SELECT workspace_id, title, created_by, is_private, created_at, updated_at FROM notes_by_id WHERE note_id = ?`,
		toGocqlUUID(noteID),
	).WithContext(ctx).Scan(&workspaceID, &title, &createdBy, &isPrivate, &createdAt, &updatedAt)
	if err == gocql.ErrNotFound {
		return uuid.Nil, "", uuid.Nil, false, time.Time{}, time.Time{}, errors.New("note not found")
	}
	if err != nil {
		return uuid.Nil, "", uuid.Nil, false, time.Time{}, time.Time{}, fmt.Errorf("get note: %w", err)
	}

	return fromGocqlUUID(workspaceID), title, fromGocqlUUID(createdBy), isPrivate, createdAt, updatedAt, nil
}
