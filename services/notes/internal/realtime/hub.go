package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/Ajay01103/go-notion/notes/internal/service"
)

// Hub manages WebSocket rooms for note collaboration.
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
	name   string
	color  string
	avatar string
}

// roomMessage carries a raw JSON frame from one client to be broadcast to peers.
type roomMessage struct {
	data   []byte
	sender *client // exact connection that sent this frame — used for fan-out skip
	userID uuid.UUID // kept for logging and persistence attribution
}

// room holds all connected clients for a single note and serialises access.
type room struct {
	noteID  uuid.UUID
	svc     *service.Service
	logger  *zap.Logger
	clients map[*client]struct{}

	// Bug-1: join is buffered (capacity 32) to match leave.
	// HandleNoteWS calls rm.join <- c from the HTTP handler goroutine. If
	// run() is busy fanning out a large broadcast, an unbuffered join blocks
	// that goroutine. The client's WS is open but it hasn't been registered
	// yet, so it misses the init message. In the worst case readPump also
	// blocks trying to send to r.broadcast, creating a two-goroutine standstill.
	join      chan *client
	leave     chan *client
	broadcast chan roomMessage
	onEmpty   func()

	mu sync.Mutex
	// currentValue is the latest full Yoopta content value as raw JSON.
	// Sent to new joiners as an "init" message.
	currentValue json.RawMessage
	buffer       []service.BufferedUpdate
	flushTimer   *time.Timer
}

// ── Wire message types ────────────────────────────────────────────────────────

type initMessage struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value,omitempty"`
}

// updateMessage is kept for backwards-compat decoding only; the client now
// sends patchMessage frames for normal edits.
type updateMessage struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

// patchMessage is the block-level diff the client sends on every keystroke.
// Upserted maps block UUID → new block JSON; Deleted lists removed block UUIDs.
type patchMessage struct {
	Type     string                     `json:"type"`
	Upserted map[string]json.RawMessage `json:"upserted"`
	Deleted  []string                   `json:"deleted"`
}

// ── Presence / cursor message types ──────────────────────────────────────────

type identifyMessage struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Color  string `json:"color"`
	Avatar string `json:"avatar,omitempty"`
}

type presenceMessage struct {
	Type   string `json:"type"`
	UserID string `json:"userId"`
	Name   string `json:"name"`
	Color  string `json:"color"`
	Avatar string `json:"avatar,omitempty"`
	Joined bool   `json:"joined"`
}

type userPresenceInfo struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
	Color  string `json:"color"`
	Avatar string `json:"avatar,omitempty"`
}

type presenceListMessage struct {
	Type  string             `json:"type"`
	Users []userPresenceInfo `json:"users"`
}

type cursorMessage struct {
	Type   string  `json:"type"`
	UserID string  `json:"userId"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
}

// mergeSnapshot applies a patch onto an existing full-document snapshot stored
// as a flat JSON object (map of blockID → blockJSON). It mutates nothing; it
// returns a newly allocated json.RawMessage ready to be stored in currentValue.
//
// We operate on the raw bytes with encoding/json to avoid defining a concrete
// Go type for the Yoopta block schema, which is owned by the TypeScript client.
func mergeSnapshot(current json.RawMessage, patch patchMessage) (json.RawMessage, error) {
	// Decode current snapshot into a generic block map.
	doc := make(map[string]json.RawMessage)
	if len(current) > 0 {
		if err := json.Unmarshal(current, &doc); err != nil {
			// Snapshot corrupt — start fresh from the patch.
			doc = make(map[string]json.RawMessage)
		}
	}

	// Apply upserts.
	for id, block := range patch.Upserted {
		doc[id] = block
	}
	// Apply deletes.
	for _, id := range patch.Deleted {
		delete(doc, id)
	}

	merged, err := json.Marshal(doc)
	if err != nil {
		return current, fmt.Errorf("mergeSnapshot marshal: %w", err)
	}
	return merged, nil
}

// ── Timing / threshold constants ─────────────────────────────────────────────

const (
	// debounce is the quiet-period after the last patch before a text-only
	// flush is triggered. Structural changes (block add/delete) flush immediately.
	debounce = 400 * time.Millisecond

	// Bug-2: heartbeat constants.
	// pingInterval is how often writePump sends a WebSocket ping frame.
	// pongTimeout is the read deadline reset on each pong — if no pong arrives
	// within this window the read deadline expires, ReadMessage returns an error,
	// and readPump sends the client to the leave channel.
	pingInterval = 30 * time.Second
	pongTimeout  = 60 * time.Second

	// Bug-3 option B: flush immediately once this many updates have accumulated,
	// regardless of whether the debounce timer has fired. Limits the maximum
	// data loss window when the process crashes mid-session.
	flushThreshold = 20
)

// NewHub creates a Hub that serves WebSocket note rooms.
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

// Shutdown flushes all active rooms to the database.
// Bug-3 option C: called on SIGTERM/SIGINT so the graceful shutdown drains
// buffered edits before the process exits — covers the crash scenario where
// the debounce timer hasn't fired yet.
func (h *Hub) Shutdown(ctx context.Context) {
	h.mu.Lock()
	rooms := make([]*room, 0, len(h.rooms))
	for _, rm := range h.rooms {
		rooms = append(rooms, rm)
	}
	h.mu.Unlock()

	for _, rm := range rooms {
		rm.flush(ctx)
	}
	h.logger.Info("hub shutdown: all rooms flushed", zap.Int("rooms", len(rooms)))
}

// HandleNoteWS upgrades an HTTP request to a WebSocket connection for a note room.
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

	h.logger.Info("active websocket session established",
		zap.String("note_id", noteID.String()),
		zap.String("user_id", userID.String()),
		zap.String("remote_addr", r.RemoteAddr),
	)

	c := &client{conn: conn, send: make(chan []byte, 64), userID: userID}
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
		noteID:    noteID,
		svc:       h.svc,
		logger:    h.logger,
		clients:   make(map[*client]struct{}),
		join:      make(chan *client, 32), // Bug-1: buffered to prevent handler goroutine from blocking
		leave:     make(chan *client, 32),
		broadcast: make(chan roomMessage, 256),
		onEmpty:   func() {},
		buffer:    make([]service.BufferedUpdate, 0, 128),
	}

	// Hydrate the last persisted value from the database.
	if snapshot, _, err := rm.svc.GetYjsState(context.Background(), noteID); err != nil {
		h.logger.Error("failed to hydrate note snapshot", zap.Error(err), zap.String("note_id", noteID.String()))
	} else if len(snapshot) > 0 {
		rm.currentValue = json.RawMessage(snapshot)
		rm.logger.Info("note room hydrated",
			zap.String("note_id", noteID.String()),
			zap.Int("content_bytes", len(snapshot)),
		)
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

// run is the single-goroutine event loop for a room.
func (r *room) run() {
	for {
		select {
		case c := <-r.join:
			r.clients[c] = struct{}{}
			r.logger.Info("websocket client joined note room",
				zap.String("note_id", r.noteID.String()),
				zap.String("user_id", c.userID.String()),
				zap.Int("active_clients", len(r.clients)),
			)
			r.sendInitialState(c)

			// Send existing room members to the new client.
			users := make([]userPresenceInfo, 0)
			for other := range r.clients {
				if other == c || other.name == "" {
					continue
				}
				users = append(users, userPresenceInfo{
					UserID: other.userID.String(),
					Name:   other.name,
					Color:  other.color,
					Avatar: other.avatar,
				})
			}
			listMsg, err := json.Marshal(presenceListMessage{Type: "presence_list", Users: users})
			if err == nil {
				c.send <- listMsg
			}

		case c := <-r.leave:
			if _, ok := r.clients[c]; !ok {
				// Already removed (slow-client drop). Ignore — don't trigger onEmpty.
				continue
			}

			// If the client identified, broadcast their leave to remaining clients.
			if c.name != "" {
				leaveMsg, err := json.Marshal(presenceMessage{
					Type:   "presence",
					UserID: c.userID.String(),
					Name:   c.name,
					Color:  c.color,
					Avatar: c.avatar,
					Joined: false,
				})
				if err == nil {
					for other := range r.clients {
						if other == c {
							continue
						}
						select {
						case other.send <- leaveMsg:
						default:
							// Slow client — drop it.
							r.logger.Warn("note ws: dropping slow client",
								zap.String("note_id", r.noteID.String()),
								zap.String("user_id", other.userID.String()),
							)
							delete(r.clients, other)
							close(other.send)
							_ = other.conn.Close()
						}
					}
				}
			}

			delete(r.clients, c)
			close(c.send)
			if c.conn != nil {
				_ = c.conn.Close()
			}
			r.logger.Info("websocket client left note room",
				zap.String("note_id", r.noteID.String()),
				zap.String("user_id", c.userID.String()),
				zap.Int("active_clients", len(r.clients)),
			)
			if len(r.clients) == 0 {
				r.flush(context.Background())
				r.onEmpty()
				return
			}

		case msg := <-r.broadcast:
			// Decode just enough to identify the frame type and extract the
			// data we need for persistence — the raw frame is broadcast as-is.
			var envelope struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(msg.data, &envelope); err != nil {
				r.logger.Debug("note ws: dropping undecodable frame",
					zap.String("note_id", r.noteID.String()),
					zap.Error(err),
				)
				continue
			}

			// isStructural is set to true when the patch contains block
			// deletions or introduces blocks not present in currentValue.
			// Structural changes flush immediately (Bug-3 option A) rather
			// than waiting for the debounce timer.
			isStructural := false
			// identify, cursor, and presence messages are handled privately
			// and should not fall through to the raw-data fan-out.
			handledPrivately := false

			switch envelope.Type {
			case "patch":
				var patch patchMessage
				if err := json.Unmarshal(msg.data, &patch); err == nil {
					// Detect structural change: any deletion or any block ID
					// that doesn't exist yet in the current snapshot.
					if len(patch.Deleted) > 0 {
						isStructural = true
					}
					if !isStructural {
						r.mu.Lock()
						for id := range patch.Upserted {
							var doc map[string]json.RawMessage
							if len(r.currentValue) > 0 {
								_ = json.Unmarshal(r.currentValue, &doc)
							}
							if _, exists := doc[id]; !exists {
								isStructural = true
								break
							}
						}
						r.mu.Unlock()
					}

					r.mu.Lock()
					merged, err := mergeSnapshot(r.currentValue, patch)
					if err != nil {
						r.logger.Error("note ws: snapshot merge failed",
							zap.Error(err),
							zap.String("note_id", r.noteID.String()),
						)
					} else {
						r.currentValue = merged
					}
					r.buffer = append(r.buffer, service.BufferedUpdate{Data: r.currentValue, UserID: msg.userID})
					bufLen := len(r.buffer)
					r.mu.Unlock()

					switch {
					case isStructural:
						// Bug-3 option A: flush synchronously for structural
						// changes — block add/delete are high-value and rare;
						// the extra latency is acceptable.
						r.flush(context.Background())
					case bufLen >= flushThreshold:
						// Bug-3 option B: cap the loss window at flushThreshold
						// updates regardless of timing.
						r.flush(context.Background())
					default:
						// Normal text keystroke: debounce to batch writes.
						r.mu.Lock()
						if r.flushTimer != nil {
							r.flushTimer.Reset(debounce)
						} else {
							r.flushTimer = time.AfterFunc(debounce, func() {
								r.flush(context.Background())
							})
						}
						r.mu.Unlock()
					}
				}

			case "update":
				// Legacy full-snapshot frame — kept for forward-compat during
				// rolling deploys where an old client may still be connected.
				var incoming updateMessage
				if err := json.Unmarshal(msg.data, &incoming); err == nil && len(incoming.Value) > 0 {
					r.mu.Lock()
					r.currentValue = incoming.Value
					r.buffer = append(r.buffer, service.BufferedUpdate{Data: incoming.Value, UserID: msg.userID})
					bufLen := len(r.buffer)
					if r.flushTimer != nil {
						r.flushTimer.Reset(debounce)
					} else {
						r.flushTimer = time.AfterFunc(debounce, func() {
							r.flush(context.Background())
						})
					}
					r.mu.Unlock()
					if bufLen >= flushThreshold {
						r.flush(context.Background())
					}
				}

			case "identify":
				handledPrivately = true
				var idMsg identifyMessage
				if err := json.Unmarshal(msg.data, &idMsg); err == nil {
					msg.sender.name = idMsg.Name
					msg.sender.color = idMsg.Color
					msg.sender.avatar = idMsg.Avatar

					pr := presenceMessage{
						Type:   "presence",
						UserID: msg.sender.userID.String(),
						Name:   msg.sender.name,
						Color:  msg.sender.color,
						Avatar: msg.sender.avatar,
						Joined: true,
					}
					prBytes, err := json.Marshal(pr)
					if err == nil {
						for other := range r.clients {
							if other == msg.sender {
								continue
							}
							select {
							case other.send <- prBytes:
							default:
								// Slow client — drop it.
								r.logger.Warn("note ws: dropping slow client",
									zap.String("note_id", r.noteID.String()),
									zap.String("user_id", other.userID.String()),
								)
								delete(r.clients, other)
								close(other.send)
								_ = other.conn.Close()
							}
						}
					}
				}

			case "cursor":
				handledPrivately = true
				var cur cursorMessage
				if err := json.Unmarshal(msg.data, &cur); err == nil {
					cur.UserID = msg.sender.userID.String()
					curBytes, err := json.Marshal(cur)
					if err == nil {
						for other := range r.clients {
							if other == msg.sender {
								continue
							}
							select {
							case other.send <- curBytes:
							default:
								// Slow client — drop it.
								r.logger.Warn("note ws: dropping slow client",
									zap.String("note_id", r.noteID.String()),
									zap.String("user_id", other.userID.String()),
								)
								delete(r.clients, other)
								close(other.send)
								_ = other.conn.Close()
							}
						}
					}
				}
			}

			if handledPrivately {
				continue
			}

			// Broadcast the raw frame to all OTHER clients immediately.
			// "Other" means a different connection object — not userID comparison.
			// Two browser windows logged in as the same user have the same userID
			// but are distinct *client pointers. Filtering by userID would silently
			// drop all messages to the second window of the same user.
			sent := 0
			for c := range r.clients {
				if c == msg.sender {
					continue // skip the exact connection that sent this frame
				}
				select {
				case c.send <- msg.data:
					sent++
				default:
					// Slow client — drop it.
					r.logger.Warn("note ws: dropping slow client",
						zap.String("note_id", r.noteID.String()),
						zap.String("user_id", c.userID.String()),
					)
					delete(r.clients, c)
					close(c.send)
					_ = c.conn.Close()
				}
			}
			r.logger.Debug("note ws: broadcast sent",
				zap.String("note_id", r.noteID.String()),
				zap.String("from_user", msg.userID.String()),
				zap.String("frame_type", envelope.Type),
				zap.Int("frame_bytes", len(msg.data)),
				zap.Int("recipients", sent),
				zap.Int("total_clients", len(r.clients)),
			)
		}
	}
}

// sendInitialState sends the current in-memory value to a newly joined client.
func (r *room) sendInitialState(c *client) {
	r.mu.Lock()
	value := append(json.RawMessage(nil), r.currentValue...)
	r.mu.Unlock()

	if len(value) == 0 {
		// No content yet — send an empty init so the client knows the WS is ready.
		msg, _ := json.Marshal(initMessage{Type: "init"})
		c.send <- msg
		return
	}

	msg, err := json.Marshal(initMessage{Type: "init", Value: value})
	if err != nil {
		r.logger.Error("failed to marshal init message", zap.Error(err))
		return
	}
	c.send <- msg
}

// flush persists the buffered updates and latest snapshot to the database.
func (r *room) flush(ctx context.Context) {
	r.mu.Lock()
	if len(r.buffer) == 0 {
		r.flushTimer = nil
		r.mu.Unlock()
		return
	}
	batch := make([]service.BufferedUpdate, len(r.buffer))
	copy(batch, r.buffer)
	r.buffer = r.buffer[:0]
	snapshot := append(json.RawMessage(nil), r.currentValue...)
	r.flushTimer = nil
	r.mu.Unlock()

	if err := r.svc.BatchAppendNoteUpdates(ctx, r.noteID, batch); err != nil {
		r.logger.Error("failed to flush note updates", zap.Error(err), zap.String("note_id", r.noteID.String()))
		return
	}

	if len(snapshot) > 0 {
		if err := r.svc.UpsertYjsState(ctx, r.noteID, snapshot); err != nil {
			r.logger.Error("failed to persist note snapshot", zap.Error(err), zap.String("note_id", r.noteID.String()))
		}
	}
}

// handleClientMessage processes a single frame from a connected client.
func (r *room) handleClientMessage(c *client, payload []byte) {
	// Only accept valid JSON frames.
	if len(payload) == 0 || payload[0] != '{' {
		r.logger.Debug("note ws: dropping non-JSON frame",
			zap.String("note_id", r.noteID.String()),
			zap.String("user_id", c.userID.String()),
			zap.Int("bytes", len(payload)),
		)
		return
	}
	r.logger.Debug("note ws: queuing broadcast",
		zap.String("note_id", r.noteID.String()),
		zap.String("user_id", c.userID.String()),
		zap.Int("bytes", len(payload)),
	)
	r.broadcast <- roomMessage{data: payload, sender: c, userID: c.userID}
}

// readPump reads frames from the WebSocket and feeds them into the room loop.
// Bug-2: the initial read deadline is set here; the pong handler in writePump
// resets it on every pong so the deadline only fires when the client goes silent.
func (r *room) readPump(c *client) {
	defer func() {
		r.leave <- c
	}()

	// Set the initial read deadline. writePump's PongHandler will keep resetting
	// it. If no pong arrives within pongTimeout the deadline expires,
	// ReadMessage returns an error, and the defer sends c to r.leave.
	c.conn.SetReadDeadline(time.Now().Add(pongTimeout))

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

// writePump drains the client's send channel and writes frames to the WebSocket.
// Bug-2: a ticker sends a WebSocket ping every pingInterval. The PongHandler
// registered here resets the read deadline so readPump doesn't time out the
// connection while the client is alive but idle.
// Uses BinaryMessage for data frames to match the client's ws.binaryType =
// "arraybuffer" setting (ArrayBuffer → TextDecoder on the JS side).
func (r *room) writePump(c *client) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	// Register pong handler: each pong extends the read deadline in readPump.
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongTimeout))
		return nil
	})

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				// send channel closed by run() — client left or was dropped.
				// Send a close frame before returning so the browser knows.
				_ = c.conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			if err := c.conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			// Bug-2: send ping. If this errors the client is gone — return so
			// the goroutine exits. readPump's next read will also error (the
			// conn is broken) and send c to r.leave.
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
