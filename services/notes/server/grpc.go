package server

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/Ajay01103/go-notion/pkg/interceptor"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/Ajay01103/go-notion/notes/gen/pb"
	"github.com/Ajay01103/go-notion/notes/gen/pb/pbconnect"
	"github.com/Ajay01103/go-notion/notes/internal/service"
)

type NotesServer struct {
	pbconnect.UnimplementedNotesServiceHandler
	svc *service.Service
}

func New(svc *service.Service) *NotesServer {
	return &NotesServer{svc: svc}
}

func (s *NotesServer) CreateNote(ctx context.Context, req *connect.Request[pb.CreateNoteRequest]) (*connect.Response[pb.Note], error) {
	if req.Msg.GetWorkspaceId() == "" || req.Msg.GetTitle() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id and title are required"))
	}
	workspaceID, err := uuid.Parse(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	note, err := s.svc.CreateNote(ctx, workspaceID, userID, req.Msg.GetTitle(), req.Msg.GetIsPrivate())
	if err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	return connect.NewResponse(note), nil
}

func (s *NotesServer) GetNote(ctx context.Context, req *connect.Request[pb.GetNoteRequest]) (*connect.Response[pb.Note], error) {
	if req.Msg.GetNoteId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("note_id is required"))
	}
	noteID, err := uuid.Parse(req.Msg.GetNoteId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	note, err := s.svc.GetNote(ctx, noteID, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	return connect.NewResponse(note), nil
}

func (s *NotesServer) ListWorkspaceNotes(ctx context.Context, req *connect.Request[pb.ListWorkspaceNotesRequest]) (*connect.Response[pb.ListWorkspaceNotesResponse], error) {
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

	notes, err := s.svc.ListWorkspaceNotes(ctx, workspaceID, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	return connect.NewResponse(&pb.ListWorkspaceNotesResponse{Notes: notes}), nil
}

func (s *NotesServer) AppendNoteUpdate(ctx context.Context, req *connect.Request[pb.AppendNoteUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if req.Msg.GetNoteId() == "" || len(req.Msg.GetData()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("note_id and data are required"))
	}
	noteID, err := uuid.Parse(req.Msg.GetNoteId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	if err := s.svc.AppendNoteUpdate(ctx, noteID, userID, req.Msg.GetData()); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}
