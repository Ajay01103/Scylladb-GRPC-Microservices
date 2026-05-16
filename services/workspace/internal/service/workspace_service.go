package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Ajay01103/go-notion/workspace/gen/pb"
	"github.com/Ajay01103/go-notion/workspace/internal/authz"
)

// Service encapsulates workspace business logic
type Service struct {
	session *gocql.Session
	logger  *zap.Logger
	authz   *authz.Checker
}

// New creates a new workspace service
func New(session *gocql.Session, logger *zap.Logger) *Service {
	return &Service{
		session: session,
		logger:  logger,
		authz:   authz.New(),
	}
}

// CreateWorkspace creates a new workspace
func (s *Service) CreateWorkspace(ctx context.Context, ownerID uuid.UUID, name, slug, description, iconURL string, isPublic bool) (*pb.Workspace, error) {
	workspaceID := uuid.New()
	now := time.Now().UTC()

	// Validate slug format (alphanumeric, hyphens only)
	if err := validateSlug(slug); err != nil {
		return nil, err
	}

	// Reserve workspace name so names stay unique across all workspaces.
	casResult := map[string]interface{}{}
	nameReserved, err := s.session.Query(
		`INSERT INTO workspace_names (name, workspace_id, created_at) VALUES (?, ?, ?) IF NOT EXISTS`,
		name, workspaceID.String(), now,
	).WithContext(ctx).MapScanCAS(casResult)
	if err != nil {
		return nil, fmt.Errorf("reserve workspace name: %w", err)
	}
	if !nameReserved {
		return nil, fmt.Errorf("workspace name already exists")
	}

	// Create workspace row
	err = s.session.Query(
		`INSERT INTO workspaces (id, name, slug, description, icon_url, owner_id, is_public, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		workspaceID.String(), name, slug, description, iconURL, ownerID.String(), isPublic, now, now,
	).WithContext(ctx).Exec()
	if err != nil {
		s.logger.Error("failed to create workspace", zap.Error(err))
		_ = s.session.Query(`DELETE FROM workspace_names WHERE name = ?`, name).WithContext(ctx).Exec()
		return nil, fmt.Errorf("create workspace: %w", err)
	}

	// Reserve slug
	err = s.session.Query(
		`INSERT INTO workspace_slugs (slug, workspace_id, created_at) VALUES (?, ?, ?)`,
		slug, workspaceID.String(), now,
	).WithContext(ctx).Exec()
	if err != nil {
		s.logger.Error("failed to reserve slug", zap.Error(err))
		// Continue—slug reservation is best-effort
	}

	// Add owner membership
	err = s.session.Query(
		`INSERT INTO workspace_members (workspace_id, user_id, role, invited_by, joined_at)
		 VALUES (?, ?, ?, ?, ?)`,
		workspaceID.String(), ownerID.String(), "owner", ownerID.String(), now,
	).WithContext(ctx).Exec()
	if err != nil {
		s.logger.Error("failed to create workspace owner membership", zap.Error(err))
		return nil, fmt.Errorf("create workspace owner membership: %w", err)
	}

	// Add owner membership to user view
	err = s.session.Query(
		`INSERT INTO workspace_members_by_user (user_id, workspace_id, role, invited_by, joined_at)
		 VALUES (?, ?, ?, ?, ?)`,
		ownerID.String(), workspaceID.String(), "owner", ownerID.String(), now,
	).WithContext(ctx).Exec()
	if err != nil {
		s.logger.Error("failed to create workspace user membership", zap.Error(err))
		_ = s.session.Query(`DELETE FROM workspace_members WHERE workspace_id = ? AND user_id = ?`, workspaceID.String(), ownerID.String()).WithContext(ctx).Exec()
		_ = s.session.Query(`DELETE FROM workspace_slugs WHERE slug = ?`, slug).WithContext(ctx).Exec()
		_ = s.session.Query(`DELETE FROM workspaces WHERE id = ?`, workspaceID.String()).WithContext(ctx).Exec()
		_ = s.session.Query(`DELETE FROM workspace_names WHERE name = ?`, name).WithContext(ctx).Exec()
		return nil, fmt.Errorf("create workspace user membership: %w", err)
	}

	return &pb.Workspace{
		Id:          workspaceID.String(),
		Name:        name,
		Slug:        slug,
		Description: description,
		IconUrl:     iconURL,
		OwnerId:     ownerID.String(),
		IsPublic:    isPublic,
		CreatedAt:   timestampProto(now),
		UpdatedAt:   timestampProto(now),
		MyRole:      pb.WorkspaceRole_WORKSPACE_ROLE_OWNER,
	}, nil
}

// GetWorkspace retrieves a workspace by ID
func (s *Service) GetWorkspace(ctx context.Context, workspaceID uuid.UUID, requesterID uuid.UUID) (*pb.Workspace, error) {
	var ws pb.Workspace
	var createdAt time.Time
	var updatedAt time.Time

	err := s.session.Query(
		`SELECT id, name, slug, description, icon_url, owner_id, is_public, created_at, updated_at
		 FROM workspaces WHERE id = ?`,
		workspaceID.String(),
	).WithContext(ctx).Scan(
		&ws.Id, &ws.Name, &ws.Slug, &ws.Description, &ws.IconUrl, &ws.OwnerId,
		&ws.IsPublic, &createdAt, &updatedAt,
	)
	if err == gocql.ErrNotFound {
		return nil, errors.New("workspace not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get workspace: %w", err)
	}

	ws.CreatedAt = timestampProto(createdAt)
	ws.UpdatedAt = timestampProto(updatedAt)

	// Fetch requester's role
	role, err := s.getMemberRole(ctx, workspaceID, requesterID)
	if err != nil && err != gocql.ErrNotFound {
		return nil, err
	}
	if err == gocql.ErrNotFound && !ws.IsPublic {
		return nil, errors.New("access denied: not a member of this workspace")
	}

	if role != "" {
		ws.MyRole = authz.StringToProtoRole(role)
	} else {
		ws.MyRole = pb.WorkspaceRole_WORKSPACE_ROLE_GUEST
	}

	return &ws, nil
}

// ListMyWorkspaces lists all workspaces for a user
func (s *Service) ListMyWorkspaces(ctx context.Context, userID uuid.UUID) ([]*pb.Workspace, error) {
	workspaceRoleByID := make(map[string]string)
	workspaceIDs := make([]string, 0)

	// Primary path: user-indexed table for fast lookups.
	byUserIter := s.session.Query(
		`SELECT workspace_id, role FROM workspace_members_by_user WHERE user_id = ?`,
		userID.String(),
	).WithContext(ctx).Iter()

	var workspaceID string
	var role string

	for byUserIter.Scan(&workspaceID, &role) {
		if _, exists := workspaceRoleByID[workspaceID]; exists {
			continue
		}
		workspaceRoleByID[workspaceID] = role
		workspaceIDs = append(workspaceIDs, workspaceID)
	}

	if err := byUserIter.Close(); err != nil {
		return nil, fmt.Errorf("list user workspaces (by user): %w", err)
	}

	// Fallback path: canonical membership table. This recovers results when the
	// denormalized user index is stale or missing rows.
	membersIter := s.session.Query(
		`SELECT workspace_id, role FROM workspace_members WHERE user_id = ? ALLOW FILTERING`,
		userID.String(),
	).WithContext(ctx).Iter()

	for membersIter.Scan(&workspaceID, &role) {
		if _, exists := workspaceRoleByID[workspaceID]; exists {
			continue
		}
		workspaceRoleByID[workspaceID] = role
		workspaceIDs = append(workspaceIDs, workspaceID)

		// Best-effort backfill to keep future reads fast.
		_ = s.session.Query(
			`INSERT INTO workspace_members_by_user (user_id, workspace_id, role, invited_by, joined_at) VALUES (?, ?, ?, ?, ?)`,
			userID.String(), workspaceID, role, "", time.Now().UTC(),
		).WithContext(ctx).Exec()
	}

	if err := membersIter.Close(); err != nil {
		return nil, fmt.Errorf("list user workspaces (members fallback): %w", err)
	}

	workspaces := make([]*pb.Workspace, 0, len(workspaceIDs))
	for _, wsID := range workspaceIDs {
		parsedWorkspaceID, err := uuid.Parse(wsID)
		if err != nil {
			continue
		}

		ws, err := s.GetWorkspace(ctx, parsedWorkspaceID, userID)
		if err != nil {
			continue
		}

		if fetchedRole, exists := workspaceRoleByID[wsID]; exists {
			ws.MyRole = authz.StringToProtoRole(fetchedRole)
		}

		workspaces = append(workspaces, ws)
	}

	return workspaces, nil
}

// UpdateWorkspace updates workspace metadata
func (s *Service) UpdateWorkspace(ctx context.Context, workspaceID uuid.UUID, requesterID uuid.UUID, name, description, iconURL string, isPublic bool) (*pb.Workspace, error) {
	// Check permission
	role, err := s.getMemberRole(ctx, workspaceID, requesterID)
	if err != nil || role == "" {
		return nil, errors.New("access denied: not a member of this workspace")
	}
	if !s.authz.Can(role, authz.PermWorkspaceEdit) {
		return nil, errors.New("access denied: insufficient permissions")
	}

	now := time.Now().UTC()
	err = s.session.Query(
		`UPDATE workspaces SET name = ?, description = ?, icon_url = ?, is_public = ?, updated_at = ? WHERE id = ?`,
		name, description, iconURL, isPublic, now, workspaceID.String(),
	).WithContext(ctx).Exec()
	if err != nil {
		return nil, fmt.Errorf("update workspace: %w", err)
	}

	return s.GetWorkspace(ctx, workspaceID, requesterID)
}

// DeleteWorkspace hard-deletes a workspace
func (s *Service) DeleteWorkspace(ctx context.Context, workspaceID uuid.UUID, requesterID uuid.UUID) error {
	// Check permission (only owner)
	role, err := s.getMemberRole(ctx, workspaceID, requesterID)
	if err != nil || role != "owner" {
		return errors.New("access denied: only workspace owner can delete")
	}

	workspaceIDStr := workspaceID.String()

	var invitationTokens []string
	invitationIter := s.session.Query(
		`SELECT token FROM workspace_invitations_by_workspace WHERE workspace_id = ?`,
		workspaceIDStr,
	).WithContext(ctx).Iter()
	var invitationToken string
	for invitationIter.Scan(&invitationToken) {
		invitationTokens = append(invitationTokens, invitationToken)
	}
	if err := invitationIter.Close(); err == nil {
		for _, invitationToken := range invitationTokens {
			_ = s.session.Query(`DELETE FROM workspace_invitations WHERE token = ?`, invitationToken).WithContext(ctx).Exec()
		}
	}

	if err := s.session.Query(`DELETE FROM workspace_members_by_user WHERE workspace_id = ?`, workspaceIDStr).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("delete workspace user memberships: %w", err)
	}
	if err := s.session.Query(`DELETE FROM workspace_members WHERE workspace_id = ?`, workspaceIDStr).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("delete workspace memberships: %w", err)
	}
	if err := s.session.Query(`DELETE FROM workspace_invitations_by_workspace WHERE workspace_id = ?`, workspaceIDStr).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("delete workspace invitations by workspace: %w", err)
	}
	if err := s.session.Query(`DELETE FROM workspace_invitations WHERE token IN ?`, []string{}).WithContext(ctx).Exec(); err != nil {
		// No-op: individual invitation cleanup is handled by workspace-specific table.
		_ = err
	}
	if err := s.session.Query(`DELETE FROM workspace_slugs WHERE workspace_id = ?`, workspaceIDStr).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("delete workspace slug: %w", err)
	}
	if err := s.session.Query(`DELETE FROM workspace_names WHERE workspace_id = ?`, workspaceIDStr).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("delete workspace name: %w", err)
	}
	if err := s.session.Query(`DELETE FROM workspaces WHERE id = ?`, workspaceIDStr).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("delete workspace: %w", err)
	}

	return nil
}

// ListMembers lists all members of a workspace
func (s *Service) ListMembers(ctx context.Context, workspaceID uuid.UUID, requesterID uuid.UUID) ([]*pb.WorkspaceMember, error) {
	// Check permission
	role, err := s.getMemberRole(ctx, workspaceID, requesterID)
	if err != nil || !s.authz.Can(role, authz.PermMembersView) {
		return nil, errors.New("access denied: cannot list members")
	}

	query := `SELECT user_id, role, invited_by, joined_at FROM workspace_members WHERE workspace_id = ?`
	iter := s.session.Query(query, workspaceID.String()).WithContext(ctx).Iter()
	defer iter.Close()

	var members []*pb.WorkspaceMember
	var userID string
	var memberRole string
	var invitedBy string
	var joinedAt time.Time

	for iter.Scan(&userID, &memberRole, &invitedBy, &joinedAt) {
		members = append(members, &pb.WorkspaceMember{
			UserId:      userID,
			WorkspaceId: workspaceID.String(),
			Role:        authz.StringToProtoRole(memberRole),
			InvitedBy:   invitedBy,
			JoinedAt:    timestampProto(joinedAt),
		})
	}

	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}

	return members, nil
}

// UpdateMemberRole changes a member's role
func (s *Service) UpdateMemberRole(ctx context.Context, workspaceID, userID uuid.UUID, requesterID uuid.UUID, newRole pb.WorkspaceRole) (*pb.WorkspaceMember, error) {
	// Check permission (only owner/admin can promote)
	actorRole, err := s.getMemberRole(ctx, workspaceID, requesterID)
	if err != nil || !s.authz.Can(actorRole, authz.PermMembersPromote) {
		return nil, errors.New("access denied: cannot promote members")
	}

	// Check if actor can promote to target role
	targetRoleStr := authz.ProtoRoleToString(newRole)
	if !s.authz.CanPromoteTo(actorRole, targetRoleStr) {
		return nil, errors.New("access denied: cannot promote to this role")
	}

	now := time.Now().UTC()

	// Update in workspace_members
	err = s.session.Query(
		`UPDATE workspace_members SET role = ? WHERE workspace_id = ? AND user_id = ?`,
		targetRoleStr, workspaceID.String(), userID.String(),
	).WithContext(ctx).Exec()
	if err != nil {
		return nil, fmt.Errorf("update member role: %w", err)
	}

	// Update in workspace_members_by_user (denormalized)
	err = s.session.Query(
		`UPDATE workspace_members_by_user SET role = ? WHERE user_id = ? AND workspace_id = ?`,
		targetRoleStr, userID.String(), workspaceID.String(),
	).WithContext(ctx).Exec()
	if err != nil {
		return nil, fmt.Errorf("update member role (user view): %w", err)
	}

	return &pb.WorkspaceMember{
		UserId:      userID.String(),
		WorkspaceId: workspaceID.String(),
		Role:        newRole,
		InvitedBy:   requesterID.String(),
		JoinedAt:    timestampProto(now),
	}, nil
}

// RemoveMember removes a member from a workspace
func (s *Service) RemoveMember(ctx context.Context, workspaceID, userID uuid.UUID, requesterID uuid.UUID) error {
	// Check permission
	actorRole, err := s.getMemberRole(ctx, workspaceID, requesterID)
	if err != nil || !s.authz.Can(actorRole, authz.PermMembersRemove) {
		return errors.New("access denied: cannot remove members")
	}

	// Delete from workspace_members
	err = s.session.Query(
		`DELETE FROM workspace_members WHERE workspace_id = ? AND user_id = ?`,
		workspaceID.String(), userID.String(),
	).WithContext(ctx).Exec()
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}

	// Delete from workspace_members_by_user (denormalized)
	err = s.session.Query(
		`DELETE FROM workspace_members_by_user WHERE user_id = ? AND workspace_id = ?`,
		userID.String(), workspaceID.String(),
	).WithContext(ctx).Exec()
	if err != nil {
		return fmt.Errorf("remove member (user view): %w", err)
	}

	return nil
}

// InviteMember creates an invitation for a new member
func (s *Service) InviteMember(ctx context.Context, workspaceID uuid.UUID, inviterID uuid.UUID, email string, role pb.WorkspaceRole) (token string, err error) {
	// Check permission
	inviterRole, err := s.getMemberRole(ctx, workspaceID, inviterID)
	if err != nil || !s.authz.Can(inviterRole, authz.PermMembersInvite) {
		return "", errors.New("access denied: cannot invite members")
	}

	roleStr := authz.ProtoRoleToString(role)
	if roleStr == "" {
		return "", errors.New("invalid role")
	}

	// Generate secure random token
	token, err = generateSecureToken(32)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	now := time.Now().UTC()
	expiresAt := now.Add(7 * 24 * time.Hour) // 7 days

	// Insert into workspace_invitations
	err = s.session.Query(
		`INSERT INTO workspace_invitations (token, workspace_id, invited_email, role, invited_by, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		token, workspaceID.String(), email, roleStr, inviterID.String(), expiresAt, now,
	).WithContext(ctx).Exec()
	if err != nil {
		return "", fmt.Errorf("create invitation: %w", err)
	}

	// Insert into workspace_invitations_by_workspace (denormalized for listing)
	err = s.session.Query(
		`INSERT INTO workspace_invitations_by_workspace (workspace_id, token, invited_email, role, invited_by, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		workspaceID.String(), token, email, roleStr, inviterID.String(), expiresAt, now,
	).WithContext(ctx).Exec()
	if err != nil {
		return "", fmt.Errorf("create invitation (workspace view): %w", err)
	}

	return token, nil
}

// AcceptInvitation accepts an invitation and adds the user to the workspace
func (s *Service) AcceptInvitation(ctx context.Context, token string, userID uuid.UUID) (*pb.Workspace, error) {
	// Fetch invitation
	var workspaceID string
	var role string
	var expiresAt time.Time
	var acceptedAt *time.Time

	err := s.session.Query(
		`SELECT workspace_id, role, expires_at, accepted_at FROM workspace_invitations WHERE token = ?`,
		token,
	).WithContext(ctx).Scan(&workspaceID, &role, &expiresAt, &acceptedAt)
	if err == gocql.ErrNotFound {
		return nil, errors.New("invitation not found or expired")
	}
	if err != nil {
		return nil, fmt.Errorf("accept invitation: %w", err)
	}

	parsedWorkspaceID, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("accept invitation: invalid workspace id: %w", err)
	}

	// Check if already accepted (idempotent)
	if acceptedAt != nil {
		// Already accepted—return the workspace
		return s.GetWorkspace(ctx, parsedWorkspaceID, userID)
	}

	// Check if expired
	if time.Now().UTC().After(expiresAt) {
		return nil, errors.New("invitation has expired")
	}

	now := time.Now().UTC()

	// Mark invitation as accepted
	err = s.session.Query(
		`UPDATE workspace_invitations SET accepted_at = ? WHERE token = ?`,
		now, token,
	).WithContext(ctx).Exec()
	if err != nil {
		return nil, fmt.Errorf("mark invitation accepted: %w", err)
	}

	// Add user to workspace_members
	err = s.session.Query(
		`INSERT INTO workspace_members (workspace_id, user_id, role, invited_by, joined_at)
		 VALUES (?, ?, ?, NULL, ?)`,
		workspaceID, userID.String(), role, now,
	).WithContext(ctx).Exec()
	if err != nil {
		return nil, fmt.Errorf("add member: %w", err)
	}

	// Add user to workspace_members_by_user
	err = s.session.Query(
		`INSERT INTO workspace_members_by_user (user_id, workspace_id, role, invited_by, joined_at)
		 VALUES (?, ?, ?, NULL, ?)`,
		userID.String(), workspaceID, role, now,
	).WithContext(ctx).Exec()
	if err != nil {
		return nil, fmt.Errorf("add member (user view): %w", err)
	}

	return s.GetWorkspace(ctx, parsedWorkspaceID, userID)
}

// CheckPermission checks if a user has a specific permission in a workspace
func (s *Service) CheckPermission(ctx context.Context, userID uuid.UUID, workspaceID uuid.UUID, perm authz.Permission) (bool, string, error) {
	role, err := s.getMemberRole(ctx, workspaceID, userID)
	if err != nil && err != gocql.ErrNotFound {
		return false, "", err
	}

	if role == "" {
		return false, "", nil
	}

	allowed := s.authz.Can(role, perm)
	return allowed, role, nil
}

// Helper methods

// getMemberRole retrieves a user's role in a workspace
func (s *Service) getMemberRole(ctx context.Context, workspaceID, userID uuid.UUID) (string, error) {
	var role string
	err := s.session.Query(
		`SELECT role FROM workspace_members WHERE workspace_id = ? AND user_id = ?`,
		workspaceID.String(), userID.String(),
	).WithContext(ctx).Scan(&role)
	return role, err
}

// generateSecureToken generates a secure random token
func generateSecureToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// validateSlug validates a workspace slug format
func validateSlug(slug string) error {
	if slug == "" {
		return errors.New("slug cannot be empty")
	}
	if len(slug) > 100 {
		return errors.New("slug must be 100 characters or less")
	}
	for _, ch := range slug {
		if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-') {
			return errors.New("slug must only contain lowercase letters, numbers, and hyphens")
		}
	}
	if slug[0] == '-' || slug[len(slug)-1] == '-' {
		return errors.New("slug cannot start or end with a hyphen")
	}
	return nil
}

// timestampProto converts a time.Time to a protobuf Timestamp
func timestampProto(t time.Time) *timestamppb.Timestamp {
	return timestamppb.New(t)
}
