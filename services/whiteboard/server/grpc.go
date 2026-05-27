package server

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/Ajay01103/go-notion/pkg/interceptor"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/Ajay01103/go-notion/whiteboard/gen/pb"
	"github.com/Ajay01103/go-notion/whiteboard/gen/pb/pbconnect"
	"github.com/Ajay01103/go-notion/whiteboard/internal/service"
)

type WhiteboardServer struct {
	pbconnect.UnimplementedWhiteboardServiceHandler
	svc *service.Service
}

func New(svc *service.Service) *WhiteboardServer {
	return &WhiteboardServer{svc: svc}
}

func (s *WhiteboardServer) CreateBoard(ctx context.Context, req *connect.Request[pb.CreateBoardRequest]) (*connect.Response[pb.Board], error) {
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

	board, err := s.svc.CreateBoard(ctx, workspaceID, userID, req.Msg.GetTitle(), req.Msg.GetIsPrivate())
	if err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	return connect.NewResponse(board), nil
}

func (s *WhiteboardServer) GetBoard(ctx context.Context, req *connect.Request[pb.GetBoardRequest]) (*connect.Response[pb.Board], error) {
	if req.Msg.GetBoardId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("board_id is required"))
	}
	boardID, err := uuid.Parse(req.Msg.GetBoardId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	board, err := s.svc.GetBoard(ctx, boardID, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	return connect.NewResponse(board), nil
}

func (s *WhiteboardServer) ListWorkspaceBoards(ctx context.Context, req *connect.Request[pb.ListWorkspaceBoardsRequest]) (*connect.Response[pb.ListWorkspaceBoardsResponse], error) {
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

	boards, err := s.svc.ListWorkspaceBoards(ctx, workspaceID, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	return connect.NewResponse(&pb.ListWorkspaceBoardsResponse{Boards: boards}), nil
}

func (s *WhiteboardServer) AppendBoardOp(ctx context.Context, req *connect.Request[pb.AppendBoardOpRequest]) (*connect.Response[emptypb.Empty], error) {
	if req.Msg.GetBoardId() == "" || req.Msg.GetOpType() == "" || len(req.Msg.GetRecords()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("board_id, op_type and records are required"))
	}
	boardID, err := uuid.Parse(req.Msg.GetBoardId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	if err := s.svc.AppendBoardOp(ctx, boardID, userID, req.Msg.GetOpType(), req.Msg.GetRecords()); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}
