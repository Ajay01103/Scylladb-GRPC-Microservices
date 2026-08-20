package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Ajay01103/go-notion/notes/gen/pb"
	"github.com/Ajay01103/go-notion/notes/internal/s3"
)

type BufferedUpdate struct {
	Data   []byte
	UserID uuid.UUID
}

type Service struct {
	session   *gocql.Session
	logger    *zap.Logger
	presigner *s3.Presigner
}

func toGocqlUUID(id uuid.UUID) gocql.UUID {
	return gocql.UUID(id)
}

func fromGocqlUUID(id gocql.UUID) uuid.UUID {
	return uuid.UUID(id)
}

func New(session *gocql.Session, logger *zap.Logger, presigner *s3.Presigner) *Service {
	return &Service{session: session, logger: logger, presigner: presigner}
}

// reserveNoteTitle atomically reserves a unique title for a note in
// notes_by_workspace_and_title using a Scylla lightweight transaction
// (INSERT ... IF NOT EXISTS). On collision it retries with a numeric suffix
// (-2, -3, …) up to maxAttempts times.
// Returns the reserved title and LWT row data (note_id/created_by/is_private
// still empty — caller fills those in after generating noteID).
const maxTitleReserveAttempts = 20

func (s *Service) reserveNoteTitle(
	ctx context.Context,
	workspaceID uuid.UUID,
	noteID uuid.UUID,
	baseTitle string,
	userID uuid.UUID,
	isPrivate bool,
	now time.Time,
) (string, error) {
	for attempt := 0; attempt < maxTitleReserveAttempts; attempt++ {
		candidate := baseTitle
		if attempt > 0 {
			candidate = baseTitle + "-" + strconv.Itoa(attempt+1)
		}

		applied := false
		casResult := map[string]interface{}{}
		var err error
		applied, err = s.session.Query(
			`INSERT INTO notes_by_workspace_and_title
			 (workspace_id, title, note_id, created_by, is_private, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?) IF NOT EXISTS`,
			toGocqlUUID(workspaceID), candidate, toGocqlUUID(noteID),
			toGocqlUUID(userID), isPrivate, now, now,
		).WithContext(ctx).MapScanCAS(casResult)
		if err != nil {
			return "", fmt.Errorf("reserve note title LWT (attempt %d): %w", attempt+1, err)
		}
		if applied {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not reserve unique title after %d attempts for base %q", maxTitleReserveAttempts, baseTitle)
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

	// Atomically reserve a unique title in the lookup table. If the base title
	// is already taken, reserveNoteTitle auto-appends -2, -3, … until it wins
	// an LWT or exhausts attempts.
	finalTitle, err := s.reserveNoteTitle(ctx, workspaceID, noteID, title, userID, isPrivate, now)
	if err != nil {
		return nil, err
	}

	if err := s.session.Query(
		`INSERT INTO notes (workspace_id, note_id, title, created_by, is_private, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		toGocqlUUID(workspaceID), toGocqlUUID(noteID), finalTitle, toGocqlUUID(userID), isPrivate, now, now,
	).WithContext(ctx).Exec(); err != nil {
		return nil, fmt.Errorf("create note: %w", err)
	}

	if err := s.session.Query(
		`INSERT INTO notes_by_id (note_id, workspace_id, title, created_by, is_private, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		toGocqlUUID(noteID), toGocqlUUID(workspaceID), finalTitle, toGocqlUUID(userID), isPrivate, now, now,
	).WithContext(ctx).Exec(); err != nil {
		s.logger.Error("failed to insert notes_by_id", zap.Error(err), zap.String("note_id", noteID.String()))
	}

	return &pb.Note{
		Id:          noteID.String(),
		WorkspaceId: workspaceID.String(),
		Title:       finalTitle,
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

// noteUpdateTTLSeconds is the retention window for yjs_updates rows.
// 72 hours — updates are only needed for the audit log; yjs_state holds the
// authoritative snapshot. Rows auto-expire so the table stays bounded.
const noteUpdateTTLSeconds = 72 * 60 * 60 // 259200

func (s *Service) BatchAppendNoteUpdates(ctx context.Context, noteID uuid.UUID, updates []BufferedUpdate) error {
	batch := s.session.NewBatch(gocql.UnloggedBatch)
	for _, update := range updates {
		batch.Query(
			fmt.Sprintf(
				`INSERT INTO yjs_updates (note_id, update_id, data, user_id) VALUES (?, now(), ?, ?) USING TTL %d`,
				noteUpdateTTLSeconds,
			),
			toGocqlUUID(noteID), update.Data, toGocqlUUID(update.UserID),
		)
	}
	return s.session.ExecuteBatch(batch.WithContext(ctx))
}

func (s *Service) UpsertYjsState(ctx context.Context, noteID uuid.UUID, content []byte) error {
	now := time.Now().UTC()
	return s.session.Query(
		`INSERT INTO yjs_state (note_id, content, updated_at) VALUES (?, ?, ?)`,
		toGocqlUUID(noteID), content, now,
	).WithContext(ctx).Exec()
}

func (s *Service) GetYjsState(ctx context.Context, noteID uuid.UUID) ([]byte, time.Time, error) {
	var content []byte
	var updatedAt time.Time
	err := s.session.Query(
		`SELECT content, updated_at FROM yjs_state WHERE note_id = ? LIMIT 1`,
		toGocqlUUID(noteID),
	).WithContext(ctx).Scan(&content, &updatedAt)
	if err == gocql.ErrNotFound {
		return nil, time.Time{}, nil
	}
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("get yjs state: %w", err)
	}
	return content, updatedAt, nil
}

func (s *Service) UpdatesSinceTimestamp(ctx context.Context, noteID uuid.UUID, since time.Time) ([][]byte, error) {
	var query string
	var args []any
	if since.IsZero() {
		query = `SELECT data FROM yjs_updates WHERE note_id = ?`
		args = []any{toGocqlUUID(noteID)}
	} else {
		query = `SELECT data FROM yjs_updates WHERE note_id = ? AND update_id > minTimeuuid(?)`
		args = []any{toGocqlUUID(noteID), since}
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
		return nil, fmt.Errorf("updates since timestamp: %w", err)
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

// ListWorkspaceIDsForUser returns all workspace IDs the user is a member of.
func (s *Service) ListWorkspaceIDsForUser(ctx context.Context, userID uuid.UUID) ([]string, error) {
	iter := s.session.Query(
		`SELECT workspace_id FROM workspace_ks.workspace_members_by_user WHERE user_id = ?`,
		userID.String(),
	).WithContext(ctx).Iter()

	var wsID string
	var ids []string
	for iter.Scan(&wsID) {
		ids = append(ids, wsID)
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("list workspace ids for user: %w", err)
	}
	return ids, nil
}

// GetNoteBySlug finds a note by its title (slug) across all workspaces the
// requester belongs to. It performs one direct-partition lookup per workspace
// against notes_by_workspace_and_title (no secondary index, no ALLOW FILTERING).
// Private notes are only returned if the requester is the creator.
func (s *Service) GetNoteBySlug(ctx context.Context, slug string, requesterID uuid.UUID) (*pb.Note, string, error) {
	if slug == "" {
		return nil, "", errors.New("slug is required")
	}

	workspaceIDs, err := s.ListWorkspaceIDsForUser(ctx, requesterID)
	if err != nil {
		return nil, "", err
	}

	for _, wsIDStr := range workspaceIDs {
		wsID, err := uuid.Parse(wsIDStr)
		if err != nil {
			continue
		}

		var noteIDRaw gocql.UUID
		var createdByRaw gocql.UUID
		var isPrivate bool
		var createdAt, updatedAt time.Time

		err = s.session.Query(
			`SELECT note_id, created_by, is_private, created_at, updated_at
			 FROM notes_by_workspace_and_title
			 WHERE workspace_id = ? AND title = ?`,
			toGocqlUUID(wsID), slug,
		).WithContext(ctx).Scan(&noteIDRaw, &createdByRaw, &isPrivate, &createdAt, &updatedAt)
		if err == gocql.ErrNotFound {
			continue
		}
		if err != nil {
			return nil, "", fmt.Errorf("get note by slug (workspace %s): %w", wsIDStr, err)
		}

		createdBy := fromGocqlUUID(createdByRaw)
		// Private notes are strictly creator-only.
		if isPrivate && createdBy != requesterID {
			continue
		}

		noteID := fromGocqlUUID(noteIDRaw)
		return &pb.Note{
			Id:          noteID.String(),
			WorkspaceId: wsIDStr,
			Title:       slug,
			CreatedBy:   createdBy.String(),
			IsPrivate:   isPrivate,
			CreatedAt:   timestamppb.New(createdAt),
			UpdatedAt:   timestamppb.New(updatedAt),
		}, wsIDStr, nil
	}

	return nil, "", errors.New("note not found")
}

const (
	assetUploadURLTTL   = 15 * time.Minute
	assetDownloadURLTTL = 1 * time.Hour
)

func (s *Service) GenerateAssetUploadUrl(ctx context.Context, noteID uuid.UUID, userID uuid.UUID, name, mimeType string, sizeBytes int64) (*pb.PresignedUrlResponse, error) {
	if ok, err := s.CanAccessNote(ctx, noteID, userID); err != nil || !ok {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("access denied")
	}

	workspaceID, _, _, _, _, _, err := s.getNoteByID(ctx, noteID)
	if err != nil {
		return nil, err
	}

	assetID := uuid.New().String()
	s3Key := fmt.Sprintf("workspaces/%s/notes/%s/assets/%s_%s", workspaceID.String(), noteID.String(), assetID, name)

	uploadURL, expiresAt, err := s.presigner.PutObject(ctx, s3Key, assetUploadURLTTL)
	if err != nil {
		return nil, fmt.Errorf("generate upload url: %w", err)
	}

	return &pb.PresignedUrlResponse{
		AssetId:       assetID,
		S3Key:         s3Key,
		UploadUrl:     uploadURL,
		ExpiresAtUnix: expiresAt,
	}, nil
}

func (s *Service) RegisterNoteAsset(ctx context.Context, assetID string, noteID, userID uuid.UUID, name, mimeType string, sizeBytes int64, s3Key string) (*pb.RegisterAssetResponse, error) {
	if ok, err := s.CanAccessNote(ctx, noteID, userID); err != nil || !ok {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("access denied")
	}

	workspaceID, _, _, _, _, _, err := s.getNoteByID(ctx, noteID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	err = s.session.Query(
		`INSERT INTO note_assets (asset_id, note_id, workspace_id, name, mime_type, size_bytes, s3_key, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		assetID, toGocqlUUID(noteID), toGocqlUUID(workspaceID), name, mimeType, sizeBytes, s3Key, now,
	).WithContext(ctx).Exec()
	if err != nil {
		return nil, fmt.Errorf("register note asset: %w", err)
	}

	return &pb.RegisterAssetResponse{
		AssetId: assetID,
		S3Key:   s3Key,
	}, nil
}

type noteAssetRow struct {
	AssetID     string
	NoteID      gocql.UUID
	WorkspaceID gocql.UUID
	Name        string
	MimeType    string
	SizeBytes   int64
	S3Key       string
}

func (s *Service) GetAssetDownloadUrl(ctx context.Context, assetID string, userID uuid.UUID) (*pb.AssetDownloadUrlResponse, error) {
	var row noteAssetRow
	err := s.session.Query(
		`SELECT asset_id, note_id, workspace_id, name, mime_type, size_bytes, s3_key FROM note_assets WHERE asset_id = ?`,
		assetID,
	).WithContext(ctx).Scan(&row.AssetID, &row.NoteID, &row.WorkspaceID, &row.Name, &row.MimeType, &row.SizeBytes, &row.S3Key)
	if err == gocql.ErrNotFound {
		return nil, errors.New("asset not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get asset: %w", err)
	}

	noteID := fromGocqlUUID(row.NoteID)
	if ok, err := s.CanAccessNote(ctx, noteID, userID); err != nil || !ok {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("access denied")
	}

	downloadURL, expiresAt, err := s.presigner.GetObject(ctx, row.S3Key, assetDownloadURLTTL)
	if err != nil {
		return nil, fmt.Errorf("generate download url: %w", err)
	}

	return &pb.AssetDownloadUrlResponse{
		AssetId:       row.AssetID,
		S3Key:         row.S3Key,
		Name:          row.Name,
		MimeType:      row.MimeType,
		SizeBytes:     row.SizeBytes,
		DownloadUrl:   downloadURL,
		ExpiresAtUnix: expiresAt,
	}, nil
}
