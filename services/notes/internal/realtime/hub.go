package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/Ajay01103/go-notion/notes/internal/service"
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
	data   []byte
	userID uuid.UUID
}

type room struct {
	noteID uuid.UUID
	svc    *service.Service
	logger *zap.Logger

	clients   map[*client]struct{}
	join      chan *client
	leave     chan *client
	broadcast chan roomMessage

	mu            sync.Mutex
	buffer        []service.BufferedUpdate
	flushTimer    *time.Timer
	sinceSnapshot int
	onEmpty       func()
}

type initMessage struct {
	Type     string   `json:"type"`
	Snapshot []byte   `json:"snapshot,omitempty"`
	Updates  [][]byte `json:"updates,omitempty"`
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

func (h *Hub) HandleNoteWS(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	noteID, err := uuid.Parse(r.PathValue("noteId"))
	if err != nil {
		http.Error(w, "invalid note id", http.StatusBadRequest)
		return
	}

	ok, err := h.svc.CanAccessNote(r.Context(), noteID, userID)
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
	rm := h.getOrCreateRoom(noteID)
	rm.join <- c

	go rm.writePump(c)
	rm.readPump(c)
}

func (h *Hub) getOrCreateRoom(noteID uuid.UUID) *room {
	h.mu.Lock()
	defer h.mu.Unlock()

	if existing, ok := h.rooms[noteID]; ok {
		return existing
	}

	rm := &room{
		noteID:     noteID,
		svc:        h.svc,
		logger:     h.logger,
		clients:    make(map[*client]struct{}),
		join:       make(chan *client),
		leave:      make(chan *client),
		broadcast:  make(chan roomMessage, 128),
		onEmpty:    func() {},
		buffer:     make([]service.BufferedUpdate, 0, 128),
		flushTimer: nil,
	}
	rm.onEmpty = func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.rooms, noteID)
	}

	go rm.run()
	h.rooms[noteID] = rm
	return rm
}

func (r *room) run() {
	const debounce = 400 * time.Millisecond

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
				case c.send <- msg.data:
				default:
					delete(r.clients, c)
					close(c.send)
					_ = c.conn.Close()
				}
			}

			r.mu.Lock()
			r.buffer = append(r.buffer, service.BufferedUpdate{Data: msg.data, UserID: msg.userID})
			r.sinceSnapshot++
			if r.flushTimer != nil {
				r.flushTimer.Reset(debounce)
			} else {
				r.flushTimer = time.AfterFunc(debounce, func() {
					r.flush(context.Background(), false)
				})
			}
			r.mu.Unlock()
		}
	}
}

func (r *room) flush(ctx context.Context, forceSnapshot bool) {
	r.mu.Lock()
	if len(r.buffer) == 0 {
		r.flushTimer = nil
		r.mu.Unlock()
		return
	}

	batch := make([]service.BufferedUpdate, len(r.buffer))
	copy(batch, r.buffer)
	r.buffer = r.buffer[:0]
	shouldSnapshot := forceSnapshot || r.sinceSnapshot >= 100
	if shouldSnapshot {
		r.sinceSnapshot = 0
	}
	r.flushTimer = nil
	r.mu.Unlock()

	if err := r.svc.BatchAppendNoteUpdates(ctx, r.noteID, batch); err != nil {
		r.logger.Error("failed to flush note updates", zap.Error(err), zap.String("note_id", r.noteID.String()))
		return
	}

	if shouldSnapshot {
		last := batch[len(batch)-1].Data
		if err := r.svc.UpsertSnapshot(ctx, r.noteID, last); err != nil {
			r.logger.Error("failed to write note snapshot", zap.Error(err), zap.String("note_id", r.noteID.String()))
		}
	}
}

func (r *room) sendInitialState(c *client) {
	snapshot, after, err := r.svc.LatestSnapshot(context.Background(), r.noteID)
	if err != nil {
		r.logger.Error("failed to load latest snapshot", zap.Error(err))
		return
	}

	updates, err := r.svc.UpdatesSince(context.Background(), r.noteID, gocqlUUID(after))
	if err != nil {
		r.logger.Error("failed to load note updates", zap.Error(err))
		return
	}

	msg, err := json.Marshal(initMessage{Type: "init", Snapshot: snapshot, Updates: updates})
	if err != nil {
		r.logger.Error("failed to marshal init message", zap.Error(err))
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
		r.broadcast <- roomMessage{data: payload, userID: c.userID}
	}
}

func (r *room) writePump(c *client) {
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
			return
		}
	}
}

func gocqlUUID(id uuid.UUID) [16]byte {
	if id == uuid.Nil {
		return [16]byte{}
	}
	var out [16]byte
	copy(out[:], id[:])
	return out
}
