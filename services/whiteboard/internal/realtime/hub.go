package realtime

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/Ajay01103/go-notion/whiteboard/internal/service"
)

type Hub struct {
	logger   *zap.Logger
	svc      *service.Service
	upgrader websocket.Upgrader

	mu    sync.Mutex
	rooms map[uuid.UUID]*room
}

type client struct {
	conn   *websocket.Conn
	send   chan []byte
	userID uuid.UUID
}

type roomMessage struct {
	payload []byte
	userID  uuid.UUID
}

type room struct {
	boardID uuid.UUID
	svc     *service.Service
	logger  *zap.Logger

	clients   map[*client]struct{}
	join      chan *client
	leave     chan *client
	broadcast chan roomMessage

	mu       sync.Mutex
	pending  []service.BufferedOp
	timer    *time.Timer
	onEmpty  func()
	opsCount int
}

func NewHub(svc *service.Service, logger *zap.Logger) *Hub {
	return &Hub{
		svc:    svc,
		logger: logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(_ *http.Request) bool { return true },
		},
		rooms: make(map[uuid.UUID]*room),
	}
}

func (h *Hub) HandleBoardWS(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	boardID, err := uuid.Parse(r.PathValue("boardId"))
	if err != nil {
		http.Error(w, "invalid board id", http.StatusBadRequest)
		return
	}

	ok, err := h.svc.CanAccessBoard(r.Context(), boardID, userID)
	if err != nil {
		h.logger.Error("access check failed", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", zap.Error(err))
		return
	}

	c := &client{conn: conn, send: make(chan []byte, 32), userID: userID}
	rm := h.getOrCreateRoom(boardID)
	rm.join <- c

	go rm.writePump(c)
	rm.readPump(c)
}

func (h *Hub) getOrCreateRoom(boardID uuid.UUID) *room {
	h.mu.Lock()
	defer h.mu.Unlock()

	if existing, ok := h.rooms[boardID]; ok {
		return existing
	}

	rm := &room{
		boardID:   boardID,
		svc:       h.svc,
		logger:    h.logger,
		clients:   make(map[*client]struct{}),
		join:      make(chan *client),
		leave:     make(chan *client),
		broadcast: make(chan roomMessage, 128),
		pending:   make([]service.BufferedOp, 0, 128),
		onEmpty:   func() {},
	}
	rm.onEmpty = func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.rooms, boardID)
	}

	go rm.run()
	h.rooms[boardID] = rm
	return rm
}

func (r *room) run() {
	const debounce = 350 * time.Millisecond

	for {
		select {
		case c := <-r.join:
			r.clients[c] = struct{}{}
			r.sendInitialState(c)
		case c := <-r.leave:
			if _, ok := r.clients[c]; ok {
				delete(r.clients, c)
				close(c.send)
				_ = c.conn.Close()
			}
			if len(r.clients) == 0 {
				r.flush(context.Background(), true)
				r.onEmpty()
				return
			}
		case msg := <-r.broadcast:
			for c := range r.clients {
				if c.userID == msg.userID {
					continue
				}
				select {
				case c.send <- msg.payload:
				default:
					delete(r.clients, c)
					close(c.send)
					_ = c.conn.Close()
				}
			}

			r.mu.Lock()
			r.pending = append(r.pending, service.BufferedOp{OpType: "patch", Records: msg.payload, UserID: msg.userID})
			r.opsCount++
			if r.timer != nil {
				r.timer.Reset(debounce)
			} else {
				r.timer = time.AfterFunc(debounce, func() {
					r.flush(context.Background(), false)
				})
			}
			r.mu.Unlock()
		}
	}
}

func (r *room) flush(ctx context.Context, forceSnapshot bool) {
	r.mu.Lock()
	if len(r.pending) == 0 {
		r.timer = nil
		r.mu.Unlock()
		return
	}

	ops := make([]service.BufferedOp, len(r.pending))
	copy(ops, r.pending)
	r.pending = r.pending[:0]
	shouldSnapshot := forceSnapshot || r.opsCount >= 100
	if shouldSnapshot {
		r.opsCount = 0
	}
	r.timer = nil
	r.mu.Unlock()

	if err := r.svc.BatchAppendBoardOps(ctx, r.boardID, ops); err != nil {
		r.logger.Error("failed to flush board ops", zap.Error(err), zap.String("board_id", r.boardID.String()))
		return
	}

	if shouldSnapshot {
		last := ops[len(ops)-1].Records
		if err := r.svc.UpsertBoardSnapshot(ctx, r.boardID, last); err != nil {
			r.logger.Error("failed to write board snapshot", zap.Error(err), zap.String("board_id", r.boardID.String()))
		}
	}
}

func (r *room) sendInitialState(c *client) {
	snapshot, err := r.svc.LatestBoardSnapshot(context.Background(), r.boardID)
	if err != nil {
		r.logger.Error("failed to load board snapshot", zap.Error(err))
		return
	}
	if len(snapshot) > 0 {
		c.send <- snapshot
	}
}

func (r *room) readPump(c *client) {
	defer func() {
		r.leave <- c
	}()
	for {
		messageType, payload, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.BinaryMessage && messageType != websocket.TextMessage {
			continue
		}
		r.broadcast <- roomMessage{payload: payload, userID: c.userID}
	}
}

func (r *room) writePump(c *client) {
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
			return
		}
	}
}
