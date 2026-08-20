package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
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
	clock   int64
}

type outgoingBoardMessage struct {
	Type     string          `json:"type"`
	Clock    int64           `json:"clock,omitempty"`
	Document json.RawMessage `json:"document,omitempty"`
}

type room struct {
	boardID   uuid.UUID
	svc       *service.Service
	logger    *zap.Logger
	clients   map[*client]struct{}
	join      chan *client
	leave     chan *client
	broadcast chan roomMessage
	onEmpty   func()

	mu           sync.Mutex
	buffer       []service.BufferedOp
	flushTimer   *time.Timer
	currentDoc   []byte
	currentClock int64
}

func NewHub(svc *service.Service, logger *zap.Logger) *Hub {
	allowedOrigin := os.Getenv("FRONTEND_ORIGIN")
	if allowedOrigin == "" {
		allowedOrigin = "http://localhost:3000"
	}
	return &Hub{
		svc:    svc,
		logger: logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				return origin == "" || strings.TrimRight(origin, "/") == strings.TrimRight(allowedOrigin, "/")
			},
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

	h.logger.Info("active websocket session established",
		zap.String("board_id", boardID.String()),
		zap.String("user_id", userID.String()),
		zap.String("remote_addr", r.RemoteAddr),
	)

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
		onEmpty:   func() {},
		buffer:    make([]service.BufferedOp, 0, 128),
	}
	if err := rm.hydrate(context.Background()); err != nil {
		h.logger.Error("failed to load board state", zap.Error(err), zap.String("board_id", boardID.String()))
	} else {
		rm.logger.Info("board room hydrated",
			zap.String("board_id", boardID.String()),
			zap.Int64("board_clock", rm.currentClock),
		)
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
	const debounce = 400 * time.Millisecond

	for {
		select {
		case c := <-r.join:
			r.clients[c] = struct{}{}
			r.logger.Info("websocket client joined board room",
				zap.String("board_id", r.boardID.String()),
				zap.String("user_id", c.userID.String()),
				zap.Int("active_clients", len(r.clients)),
			)
			r.sendInitialState(c)

		case c := <-r.leave:
			if _, ok := r.clients[c]; ok {
				delete(r.clients, c)
				close(c.send)
				_ = c.conn.Close()
				r.logger.Info("websocket client left board room",
					zap.String("board_id", r.boardID.String()),
					zap.String("user_id", c.userID.String()),
					zap.Int("active_clients", len(r.clients)),
				)
			}
			if len(r.clients) == 0 {
				r.onEmpty()
				return
			}

		case msg := <-r.broadcast:
			r.mu.Lock()
			r.buffer = append(r.buffer, service.BufferedOp{Records: msg.payload, UserID: msg.userID})
			r.currentDoc = msg.payload
			r.currentClock = msg.clock
			if r.flushTimer != nil {
				r.flushTimer.Reset(debounce)
			} else {
				r.flushTimer = time.AfterFunc(debounce, func() {
					r.flush(context.Background())
				})
			}
			r.mu.Unlock()

			outgoing, err := json.Marshal(outgoingBoardMessage{
				Type:     "snapshot",
				Clock:    msg.clock,
				Document: json.RawMessage(msg.payload),
			})
			if err != nil {
				r.logger.Error("failed to marshal board snapshot message", zap.Error(err), zap.String("board_id", r.boardID.String()))
				continue
			}

			for c := range r.clients {
				if c.userID == msg.userID {
					continue
				}
				select {
				case c.send <- outgoing:
				default:
					delete(r.clients, c)
					close(c.send)
					_ = c.conn.Close()
				}
			}
		}
	}
}

func (r *room) hydrate(ctx context.Context) error {
	document, opClock, found, err := r.svc.LoadBoardState(ctx, r.boardID)
	if err != nil {
		return err
	}

	r.mu.Lock()
	r.currentDoc = document
	r.currentClock = opClock
	r.mu.Unlock()

	startClock := opClock
	if !found {
		startClock = 0
	}

	ops, err := r.svc.LoadBoardOps(ctx, r.boardID, startClock)
	if err != nil {
		return err
	}

	if len(ops) == 0 {
		return nil
	}

	r.mu.Lock()
	for _, op := range ops {
		r.currentDoc = op.Data
		r.currentClock = op.Clock
	}
	r.mu.Unlock()
	return nil
}

func (r *room) sendInitialState(c *client) {
	r.mu.Lock()
	document := append([]byte(nil), r.currentDoc...)
	clock := r.currentClock
	r.mu.Unlock()

	if len(document) == 0 {
		return
	}

	msg, err := json.Marshal(outgoingBoardMessage{
		Type:     "init",
		Clock:    clock,
		Document: json.RawMessage(document),
	})
	if err != nil {
		r.logger.Error("failed to marshal board init message", zap.Error(err))
		return
	}
	c.send <- msg
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
		r.handleClientMessage(c, payload)
	}
}

type ackMessage struct {
	Type  string `json:"type"`
	Clock int64  `json:"clock"`
}

type incomingOpMessage struct {
	Type     string          `json:"type"`
	Clock    int64           `json:"clock"`
	Data     json.RawMessage `json:"data"`
	Document json.RawMessage `json:"document"`
}

func parseIncomingPayload(payload []byte) (int64, []byte) {
	var msg incomingOpMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		clock := time.Now().UnixNano()
		return clock, payload
	}

	if len(msg.Document) > 0 {
		clock := msg.Clock
		if clock == 0 {
			clock = time.Now().UnixNano()
		}
		return clock, msg.Document
	}

	if len(msg.Data) > 0 {
		clock := msg.Clock
		if clock == 0 {
			clock = time.Now().UnixNano()
		}

		var dataString string
		if err := json.Unmarshal(msg.Data, &dataString); err == nil && dataString != "" {
			data := []byte(dataString)
			return clock, data
		}

		return clock, msg.Data
	}

	clock := msg.Clock
	if clock == 0 {
		clock = time.Now().UnixNano()
	}
	return clock, payload
}

func (r *room) handleClientMessage(c *client, payload []byte) {
	opClock, snapshotPayload := parseIncomingPayload(payload)

	r.broadcast <- roomMessage{payload: snapshotPayload, userID: c.userID, clock: opClock}

	ack := ackMessage{Type: "ack", Clock: opClock}
	if b, err := json.Marshal(ack); err == nil {
		select {
		case c.send <- b:
		default:
			_ = c.conn.WriteMessage(websocket.TextMessage, b)
		}
	}
}

func (r *room) flush(ctx context.Context) {
	r.mu.Lock()
	if len(r.buffer) == 0 {
		r.flushTimer = nil
		r.mu.Unlock()
		return
	}

	batch := make([]service.BufferedOp, len(r.buffer))
	copy(batch, r.buffer)
	r.buffer = r.buffer[:0]
	document := append([]byte(nil), r.currentDoc...)
	opClock := r.currentClock
	r.flushTimer = nil
	r.mu.Unlock()

	if err := r.svc.BatchAppendBoardOps(ctx, r.boardID, batch); err != nil {
		r.logger.Error("failed to flush board ops", zap.Error(err), zap.String("board_id", r.boardID.String()))
		return
	}

	if len(document) == 0 {
		return
	}

	if err := r.svc.UpsertBoardState(ctx, r.boardID, document, opClock); err != nil {
		r.logger.Error("failed to upsert board state", zap.Error(err), zap.String("board_id", r.boardID.String()))
	}
}

func (r *room) writePump(c *client) {
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
			return
		}
	}
}
