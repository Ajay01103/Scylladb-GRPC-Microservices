package server

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/Ajay01103/go-notion/pkg/interceptor"
	pb "github.com/Ajay01103/go-notion/workspace/gen/pb"
	"github.com/Ajay01103/go-notion/workspace/gen/pb/pbconnect"
	"github.com/Ajay01103/go-notion/workspace/internal/authz"
	"github.com/Ajay01103/go-notion/workspace/internal/service"
	"github.com/google/uuid"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// WorkspaceServer implements the pbconnect.WorkspaceServiceHandler interface
type WorkspaceServer struct {
	pbconnect.UnimplementedWorkspaceServiceHandler
	svc *service.Service
}

// New creates a new Connect WorkspaceServer instance
func New(svc *service.Service) *WorkspaceServer {
	return &WorkspaceServer{svc: svc}
}

// CreateWorkspace creates a new workspace for the authenticated user
func (s *WorkspaceServer) CreateWorkspace(ctx context.Context, req *connect.Request[pb.CreateWorkspaceRequest]) (*connect.Response[pb.Workspace], error) {
	if req.Msg.GetName() == "" || req.Msg.GetSlug() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name and slug are required"))
	}

	// Extract authenticated user from context
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	ws, err := s.svc.CreateWorkspace(ctx, userID, req.Msg.GetName(), req.Msg.GetSlug(), req.Msg.GetDescription(), req.Msg.GetIconUrl(), req.Msg.GetIsPublic())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(ws), nil
}

// GetWorkspace retrieves a workspace by ID
func (s *WorkspaceServer) GetWorkspace(ctx context.Context, req *connect.Request[pb.GetWorkspaceRequest]) (*connect.Response[pb.Workspace], error) {
	if req.Msg.GetWorkspaceId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}

	workspaceID, err := uuid.Parse(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	ws, err := s.svc.GetWorkspace(ctx, workspaceID, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	return connect.NewResponse(ws), nil
}

// UpdateWorkspace updates workspace metadata
func (s *WorkspaceServer) UpdateWorkspace(ctx context.Context, req *connect.Request[pb.UpdateWorkspaceRequest]) (*connect.Response[pb.Workspace], error) {
	if req.Msg.GetWorkspaceId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}

	workspaceID, err := uuid.Parse(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	ws, err := s.svc.UpdateWorkspace(ctx, workspaceID, userID, req.Msg.GetName(), req.Msg.GetDescription(), req.Msg.GetIconUrl(), req.Msg.GetIsPublic())
	if err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}

	return connect.NewResponse(ws), nil
}

// DeleteWorkspace soft-deletes a workspace
func (s *WorkspaceServer) DeleteWorkspace(ctx context.Context, req *connect.Request[pb.DeleteWorkspaceRequest]) (*connect.Response[emptypb.Empty], error) {
	if req.Msg.GetWorkspaceId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}

	workspaceID, err := uuid.Parse(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	err = s.svc.DeleteWorkspace(ctx, workspaceID, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// ListMyWorkspaces lists all workspaces for the authenticated user
func (s *WorkspaceServer) ListMyWorkspaces(ctx context.Context, req *connect.Request[pb.ListMyWorkspacesRequest]) (*connect.Response[pb.ListMyWorkspacesResponse], error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	workspaces, err := s.svc.ListMyWorkspaces(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	trimmedWorkspaces := make([]*pb.Workspace, 0, len(workspaces))
	for _, workspace := range workspaces {
		trimmedWorkspaces = append(trimmedWorkspaces, &pb.Workspace{
			Id:   workspace.GetId(),
			Name: workspace.GetName(),
		})
	}

	return connect.NewResponse(&pb.ListMyWorkspacesResponse{
		Workspaces: trimmedWorkspaces,
	}), nil
}

// ListMembers lists all members of a workspace
func (s *WorkspaceServer) ListMembers(ctx context.Context, req *connect.Request[pb.ListMembersRequest]) (*connect.Response[pb.ListMembersResponse], error) {
	if req.Msg.GetWorkspaceId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}

	workspaceID, err := uuid.Parse(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	members, err := s.svc.ListMembers(ctx, workspaceID, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}

	return connect.NewResponse(&pb.ListMembersResponse{
		Members: members,
	}), nil
}

// UpdateMemberRole changes a member's role
func (s *WorkspaceServer) UpdateMemberRole(ctx context.Context, req *connect.Request[pb.UpdateMemberRoleRequest]) (*connect.Response[pb.WorkspaceMember], error) {
	if req.Msg.GetWorkspaceId() == "" || req.Msg.GetUserId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id and user_id are required"))
	}

	workspaceID, err := uuid.Parse(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	targetUserID, err := uuid.Parse(req.Msg.GetUserId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	requesterID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	member, err := s.svc.UpdateMemberRole(ctx, workspaceID, targetUserID, requesterID, req.Msg.GetNewRole())
	if err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}

	return connect.NewResponse(member), nil
}

// RemoveMember removes a member from a workspace
func (s *WorkspaceServer) RemoveMember(ctx context.Context, req *connect.Request[pb.RemoveMemberRequest]) (*connect.Response[emptypb.Empty], error) {
	if req.Msg.GetWorkspaceId() == "" || req.Msg.GetUserId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id and user_id are required"))
	}

	workspaceID, err := uuid.Parse(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	targetUserID, err := uuid.Parse(req.Msg.GetUserId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	requesterID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	err = s.svc.RemoveMember(ctx, workspaceID, targetUserID, requesterID)
	if err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// InviteMember creates an invitation for a new member
func (s *WorkspaceServer) InviteMember(ctx context.Context, req *connect.Request[pb.InviteMemberRequest]) (*connect.Response[pb.InviteMemberResponse], error) {
	if req.Msg.GetWorkspaceId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}

	workspaceID, err := uuid.Parse(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	inviterID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	token, err := s.svc.InviteMember(ctx, workspaceID, inviterID, req.Msg.GetInvitedEmail(), req.Msg.GetRole())
	if err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}

	return connect.NewResponse(&pb.InviteMemberResponse{
		InvitationId: "",
		Token:        token,
	}), nil
}

// AcceptInvitation accepts an invitation and adds the user to the workspace
func (s *WorkspaceServer) AcceptInvitation(ctx context.Context, req *connect.Request[pb.AcceptInvitationRequest]) (*connect.Response[pb.AcceptInvitationResponse], error) {
	if req.Msg.GetToken() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("token is required"))
	}

	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	ws, err := s.svc.AcceptInvitation(ctx, req.Msg.GetToken(), userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	return connect.NewResponse(&pb.AcceptInvitationResponse{
		Workspace: ws,
	}), nil
}

// RejectInvitation rejects an invitation and invalidates the token.
func (s *WorkspaceServer) RejectInvitation(ctx context.Context, req *connect.Request[pb.RejectInvitationRequest]) (*connect.Response[emptypb.Empty], error) {
	if req.Msg.GetToken() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("token is required"))
	}

	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	if err := s.svc.RejectInvitation(ctx, req.Msg.GetToken(), userID); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// CheckPermission checks if a user has a specific permission (internal RPC)
func (s *WorkspaceServer) CheckPermission(ctx context.Context, req *connect.Request[pb.CheckPermissionRequest]) (*connect.Response[pb.CheckPermissionResponse], error) {
	if req.Msg.GetUserId() == "" || req.Msg.GetWorkspaceId() == "" || req.Msg.GetPermission() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id, workspace_id, and permission are required"))
	}

	userID, err := uuid.Parse(req.Msg.GetUserId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	workspaceID, err := uuid.Parse(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	allowed, roleStr, err := s.svc.CheckPermission(ctx, userID, workspaceID, authz.Permission(req.Msg.GetPermission()))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.CheckPermissionResponse{
		Allowed: allowed,
		Role:    authz.StringToProtoRole(roleStr),
	}), nil
}
