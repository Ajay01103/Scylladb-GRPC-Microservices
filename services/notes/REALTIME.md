# Notes Service — Real-time Collaboration

This document covers the entire real-time collaboration stack: how a browser tab
connects, how the editor syncs changes with peers, how the server manages rooms,
how authentication works over WebSocket, and how everything lands in the database.

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Authentication — WS Ticket Flow](#authentication--ws-ticket-flow)
3. [Client Connection Lifecycle](#client-connection-lifecycle)
4. [Editor Sync — How Yoopta Changes Are Sent](#editor-sync--how-yoopta-changes-are-sent)
5. [Server Hub — Deep Dive](#server-hub--deep-dive)
6. [Database Schema](#database-schema)
7. [Persistence Strategy — Buffering and Flush](#persistence-strategy--buffering-and-flush)
8. [Message Wire Format](#message-wire-format)
9. [Reconnection and Abort Handling](#reconnection-and-abort-handling)
10. [End-to-End Flow Diagram](#end-to-end-flow-diagram)

---

## Architecture Overview

```
Browser (Next.js)
│
│  POST /ws/ticket  ──────────────────────────────────────────┐
│  GET  /ws/notes/{noteId}?ticket=…  ─────────────────────┐  │
│                                                          │  │
▼                                                          ▼  ▼
board-socket-manager.ts          Notes Service  (port 9092)
NoteView (note-view.tsx)  ◄────► hub.go  ◄────►  ScyllaDB
Yoopta Editor
```

The notes service is a single Go binary that serves:

- **ConnectRPC (gRPC-web)** — CRUD operations (create, get, list notes)
- **REST POST /ws/ticket** — issues a short-lived auth token
- **WebSocket GET /ws/notes/{noteId}** — real-time collaboration channel

All three share the same `http.ServeMux` and are multiplexed via `h2c`
(HTTP/2 cleartext) so HTTP/1.1, HTTP/2, and WebSocket all work on one port.

---

## Authentication — WS Ticket Flow

WebSocket connections cannot carry custom `Authorization` headers. The browser
works around this by exchanging a JWT for a short-lived **ticket** before
opening the socket.

### Step 1 — Issue a ticket (REST)

```
POST /ws/ticket
Authorization: Bearer <jwt>

200 OK
{ "ticket": "3f2e1d0c-…" }
```

**Server side** (`cmd/main.go` → `service.IssueWSTicket`):

1. `authenticateHTTP` strips the `Bearer ` prefix and calls
   `jwksVerifier.Verify` — validates the JWT signature against the JWKS
   endpoint and extracts `claims.Subject` (the user UUID).
2. `IssueWSTicket` generates `uuid.NewString()` as the ticket, then runs:

   ```cql
   INSERT INTO ws_tickets (ticket, user_id, created_at)
   VALUES (?, ?, ?)
   USING TTL 120
   ```

   The row self-destructs after **120 seconds** even if never redeemed —
   no cleanup job needed.

3. The ticket UUID is returned as JSON.

### Step 2 — Redeem the ticket (WebSocket upgrade)

```
GET /ws/notes/{noteId}?ticket=3f2e1d0c-…
```

**Server side** (`cmd/main.go` → `service.RedeemWSTicket`):

1. `SELECT user_id FROM ws_tickets WHERE ticket = ?` — fetches the user.
2. **Atomic delete with LWT**:
   ```cql
   DELETE FROM ws_tickets WHERE ticket = ? IF EXISTS
   ```
   `MapScanCAS` returns `applied = false` if the row was already gone,
   meaning another concurrent redeem already consumed it.
   This prevents **replay attacks** — one ticket, one connection, ever.
3. Returns the `user_id`; the HTTP handler passes it straight into
   `notesHub.HandleNoteWS`.

### Why LWT (Lightweight Transaction)?

A plain `DELETE` after `SELECT` has a race window: two concurrent requests
could both pass the `SELECT` and both succeed. The `IF EXISTS` conditional
delete is evaluated atomically by ScyllaDB's Paxos layer — only one caller
gets `applied = true`.

---

## Client Connection Lifecycle

### `board-socket-manager.ts`

This is a module-level singleton (plain JS Maps, not React state). It manages
WebSocket rooms by `roomId` (the note UUID). All tabs/components pointing at
the same note share one socket.

**Key data structures:**

| Map | Key | Value |
|-----|-----|-------|
| `sockets` | `roomId` | `SocketEntry` — active `WebSocket` + retry state |
| `pendingConnections` | `roomId` | `PendingConnection` — in-flight `connectRoom` promise |
| `connectionTokens` | `roomId` | Monotonic `number` — bumped on every new connect attempt |
| `msgListeners` | `roomId` | `Set` of message handler callbacks |
| `stateListeners` | `roomId` | `Set` of state change callbacks |
| `sendQueue` | `roomId` | Buffered messages queued while socket is connecting |

### `connectRoom(opts)`

```
connectRoom called
│
├─ Already OPEN socket? → return it immediately
├─ Already a pending connection? → return its promise (deduplication)
├─ Socket in CONNECTING state? → wrap in a one-shot open/error listener
│
└─ New connection:
   1. Bump connectionToken  (invalidates previous in-flight attempts)
   2. Create AbortController
   3. Call opts.fetchTicket(signal)  ← REST POST /ws/ticket
   4. If aborted or token stale → throw AbortError "connection superseded"
   5. Build WebSocket URL with ?ticket=…
   6. new WebSocket(url)
   7. On open  → flush sendQueue, notify "open"
   8. On close → if abnormal, scheduleReconnect with exponential backoff
   9. On error → notify "error"
```

**Token-based stale-connection guard:** every `connectRoom` call increments
`connectionTokens[roomId]`. Every async step after `fetchTicket` checks
`connectionTokens.get(roomId) !== token` — if another `connectRoom` or
`disconnectRoom` ran in the meantime, the stale attempt self-terminates.

### `disconnectRoom(roomId)`

```
1. Bump connectionToken → invalidates any ongoing connectRoom
2. abort pendingConnection.controller → cancels in-flight fetchTicket fetch
3. clearTimeout(retryTimer)
4. ws.close(1000, "client disconnect")
5. Delete from sockets, sendQueue
6. Notify "closed"
```

Called from `NoteView`'s `useEffect` cleanup. In React StrictMode, the
component mounts → cleanup → remounts, so `disconnectRoom` fires once before
the real mount. The resulting `AbortError` is expected and is now silently
swallowed (only non-abort errors are logged).

---

## Editor Sync — How Yoopta Changes Are Sent

### The problem with Yoopta's `onChange` prop

Yoopta Editor fires `onChange` for **internal housekeeping** (selection moves,
path tracking) as well as real content changes. It also doesn't consistently
fire for programmatic updates (`setEditorValue`). The code bypasses the prop
entirely and subscribes to Yoopta's internal `"change"` event:

```ts
editor.on("change", handler)
```

The handler receives `payload.value` — the full `YooptaContentValue` after
each change.

### Outbound path (local edit → server)

```
User types in Yoopta
│
▼
editor "change" event fires
│
▼
handler in handleEditorReady (note-view.tsx)
│
├─ applyingRemote.current === true?  → skip (we're applying a remote update, don't echo)
├─ JSON.stringify(value) === lastRemoteSnapshotRef.current? → skip (echo guard)
│
▼
sendRoomMessage(noteId, JSON.stringify({ type: "update", value }))
│
▼
board-socket-manager: ws.send(data) or enqueue if not OPEN yet
```

### Inbound path (remote update → editor)

```
WebSocket message arrives
│
▼
addMessageListener callback (note-view.tsx)
│
▼
Parse JSON → { type: "init" | "update", value: YooptaContentValue }
│
▼
applyRemoteValue(value)
│
├─ No editor mounted yet? → push to pendingMessages[]
│
▼
applyingRemote.current = true   ← blocks echo-back
│
├─ Structural change? (blocks added/removed)
│   └─ editor.setEditorValue(value)
│
└─ Text-only change?
    └─ For each changed block:
        slateEd.children = newBlock.value
        slateEd.onChange()         ← pushes into Slate directly
│
▼
lastRemoteSnapshotRef.current = JSON.stringify(editor.getEditorValue())
│
▼
Promise.resolve().then(() => { applyingRemote.current = false })
  ↑ microtask delay — lets Yoopta finish its synchronous onChange cascade
    before we re-enable outbound sends
```

### Why two different apply paths?

Yoopta wraps each block in its own Slate editor instance. `setEditorValue`
creates/destroys these instances for structural changes. But for pure text
edits it doesn't update existing Slate instances because Slate v0.71+ treats
the `value` prop as initial-only. The direct `slateEd.children =` + `onChange()`
path reaches into Yoopta's `blockEditorsMap` to push the change into the live
Slate instance, bypassing this limitation.

### Pending messages at mount

If a WebSocket `init` message arrives before the Yoopta editor DOM has mounted
(common on slow connections), `applyRemoteValue` pushes it into
`pendingMessages[]`. The `handleEditorReady` callback drains this queue when
the editor is ready, taking only the **last** entry (most recent snapshot wins).

---

## Server Hub — Deep Dive

### Structures

```go
Hub
├── rooms: map[uuid.UUID]*room   // one entry per active note
└── mu: sync.Mutex               // guards rooms map

room
├── clients: map[*client]struct{}   // all connected sockets
├── currentValue: json.RawMessage   // latest full Yoopta snapshot (in-memory)
├── buffer: []BufferedUpdate        // unsaved updates since last flush
├── flushTimer: *time.Timer         // debounce timer for DB writes
│
├── join:      chan *client          // capacity: unbuffered
├── leave:     chan *client          // capacity: 32
└── broadcast: chan roomMessage      // capacity: 256
```

### `getOrCreateRoom`

Called under `Hub.mu` lock. If no room exists for the note:

1. Allocates and initialises the room struct.
2. **Hydrates from DB**: calls `svc.GetYjsState(noteID)` — reads `yjs_state`
   to load the last persisted snapshot into `room.currentValue`. New joiners
   immediately get the most recent content even if they're the first client
   after a quiet period.
3. Sets `onEmpty` to delete the room from the Hub map when all clients leave.
4. Starts `go rm.run()` — the room's single goroutine event loop.

### `room.run()` — the event loop

This is the heart of the real-time system. It is the **only goroutine** that
touches `room.clients`, ensuring no data races without per-client locking.

```
for {
    select {

    case c := <-r.join:
        r.clients[c] = struct{}{}
        r.sendInitialState(c)          // sends current snapshot as "init"

    case c := <-r.leave:
        delete(r.clients, c)
        close(c.send)
        c.conn.Close()
        if len(r.clients) == 0:
            r.flush(ctx)               // final flush before room teardown
            r.onEmpty()                // remove from Hub map
            return                     // goroutine exits

    case msg := <-r.broadcast:
        // 1. Parse incoming frame
        if msg.type == "update":
            r.currentValue = msg.value  // update in-memory snapshot
            r.buffer = append(r.buffer, BufferedUpdate{…})
            r.flushTimer.Reset(400ms)   // or start new timer

        // 2. Fan-out to all OTHER clients
        for c := range r.clients:
            if c.userID == msg.userID: continue   // skip sender
            select:
            case c.send <- msg.data:              // non-blocking send
            default:
                // slow client — drop connection immediately
                delete(r.clients, c)
                close(c.send)
                c.conn.Close()
    }
}
```

**Important design decisions:**

- **No Yjs CRDT on the server.** The server treats `value` as an opaque JSON
  blob. The full Yoopta content value is broadcast as-is. Last-writer-wins
  semantics apply — the most recent update from any client overwrites
  `currentValue`. This is simpler and works well for low-concurrent editing
  (typical for a note-taking app).

- **The `yjs_updates` table name is a naming artefact.** The schema was laid
  out for potential Yjs integration but the actual data stored is the
  serialised Yoopta `YooptaContentValue` (JSON), not binary Yjs updates.

- **Slow client policy**: if a client's `send` channel (capacity 64) is full,
  the room drops that client rather than blocking the broadcast. This protects
  all other clients from one slow connection.

### `sendInitialState`

When a client joins, it immediately receives the current in-memory snapshot:

```go
msg, _ := json.Marshal(initMessage{Type: "init", Value: r.currentValue})
c.send <- msg
```

If `currentValue` is empty (brand new note), an `{"type":"init"}` with no
`value` field is sent so the client knows the WebSocket is ready.

### `readPump` and `writePump`

Each client has **two goroutines**:

- `readPump` (runs on the calling goroutine of `HandleNoteWS`): blocks on
  `conn.ReadMessage()` and feeds frames into `r.broadcast`. On error (client
  closed, network drop), it defers `r.leave <- c` to trigger cleanup.

- `writePump` (goroutine): ranges over `c.send` and calls
  `conn.WriteMessage(BinaryMessage, msg)`. It exits when `c.send` is closed
  (which `room.run()` does on leave/drop).

Messages are sent as `BinaryMessage` even though the content is JSON text.
This matches the client's `ws.binaryType = "arraybuffer"` setting — the browser
receives an `ArrayBuffer`, which `note-view.tsx` decodes with `TextDecoder`
before parsing as JSON.

---

## Database Schema

All tables live in the `notes_ks` keyspace.

### `notes` — workspace-partitioned index

```cql
PRIMARY KEY (workspace_id, note_id)
CLUSTERING ORDER BY (note_id ASC)
```

Used by `ListWorkspaceNotes` — efficiently fetches all notes for a workspace
with a single partition scan. `workspace_id` is the partition key, so all notes
in one workspace live on the same ScyllaDB vnodes.

### `notes_by_id` — lookup by note UUID

```cql
PRIMARY KEY (note_id)
```

Secondary denormalised table. Written on every `CreateNote` alongside `notes`.
Used by `GetNote`, `CanAccessNote`, and all realtime access checks.
Since ScyllaDB (and Cassandra) don't support secondary indexes efficiently for
low-cardinality lookups, this explicit denormalisation avoids allow-filtering.

### `yjs_updates` — append-only update log

```cql
PRIMARY KEY (note_id, update_id)   -- update_id is TIMEUUID
CLUSTERING ORDER BY (update_id ASC)
USING TimeWindowCompactionStrategy (1 day window)
```

Every write through `BatchAppendNoteUpdates` inserts a new row with `now()`
as the `update_id` (a ScyllaDB server-generated TIMEUUID). The append-only
model means no row contention — ScyllaDB can handle high write throughput
without tombstones or conflicts.

`TimeWindowCompactionStrategy` groups SSTables by time window, which means
old update rows compact together efficiently and recent writes stay hot.

`UpdatesSinceTimestamp` uses `minTimeuuid(timestamp)` to query only rows
after a given point — useful for a potential catch-up sync feature.

### `yjs_state` — materialised snapshot

```cql
PRIMARY KEY (note_id)
```

Stores the latest full `YooptaContentValue` as a `BLOB`. Written by
`UpsertYjsState` after each flush. Read once on room creation to hydrate
`room.currentValue`. This means a client joining a cold room (no active
connections) still gets the latest content immediately.

Think of `yjs_updates` as the write-ahead log and `yjs_state` as the
checkpoint. The system currently only reads from the checkpoint; the update
log exists for auditability and potential future incremental sync.

### `ws_tickets` — ephemeral auth tokens

```cql
PRIMARY KEY (ticket)
WITH default_time_to_live = 120
```

Single-column partition key — O(1) lookup by ticket. The `USING TTL 120`
means ScyllaDB auto-expires rows after 2 minutes at the storage level, so
unredeeemed tickets don't accumulate. The LWT delete-on-redeem prevents reuse.

---

## Persistence Strategy — Buffering and Flush

Writing every keystroke to the database would be massively wasteful. The hub
uses a **debounced batch write** pattern:

```
User keystroke → room.broadcast → currentValue updated → update appended to buffer
                                                          │
                                                          ▼
                                              flushTimer.Reset(400ms)
                                              (or start timer if nil)

400ms of silence:
    time.AfterFunc fires → r.flush(ctx)
        │
        ├─ BatchAppendNoteUpdates → INSERT INTO yjs_updates (batch, unlogged)
        └─ UpsertYjsState        → INSERT INTO yjs_state (latest snapshot)
```

**Unlogged batch** (`gocql.UnloggedBatch`) is used for `yjs_updates` because
all rows share the same partition key (`note_id`) — ScyllaDB can write them
in a single SSTable operation without the overhead of the batch log.

**Forced flush on last client leave**: when `len(r.clients) == 0`, `r.flush`
is called synchronously before `r.onEmpty()` removes the room. This ensures
no buffered updates are lost when the last user closes their tab.

**Timer reset vs new timer**: the code calls `flushTimer.Reset(400ms)` if a
timer already exists. This correctly debounces a rapid burst of updates into
a single flush 400ms after the last update in the burst.

---

## Message Wire Format

All messages are JSON, transported as WebSocket binary frames.

### `init` — sent by server to a newly joined client

```json
{ "type": "init", "value": { ...YooptaContentValue } }
```

`value` is omitted if the note has no content yet:

```json
{ "type": "init" }
```

### `update` — sent by client to server (and broadcast to peers)

```json
{ "type": "update", "value": { ...YooptaContentValue } }
```

The server does not validate or transform the `value` field — it stores and
rebroadcasts `incoming.Value` as `json.RawMessage` without deserialisation.
This avoids an unnecessary JSON round-trip per broadcast.

Non-JSON frames are silently dropped by `handleClientMessage` (checks
`payload[0] != '{'`).

---

## Reconnection and Abort Handling

### Exponential backoff

On abnormal close (`ev.code !== 1000`), `scheduleReconnect` queues a retry:

```
delay = min(500ms × 2^retries, 30s)
```

Retries: 0→500ms, 1→1s, 2→2s, 3→4s … capped at 30s. Each retry calls the
full `connectRoom` path including a fresh `fetchTicket` request — tickets are
single-use so a new one must be obtained for each connection attempt.

### Stale retry prevention

Before scheduling, `scheduleReconnect` checks:

```ts
const currentToken = connectionTokens.get(entry.opts.roomId)
if (currentToken !== entry.token) return
```

If the user navigated away (`disconnectRoom` bumped the token), pending retry
timers exit immediately without making any requests.

### AbortError suppression

`fetchTicket` is passed the `AbortController`'s signal. When `disconnectRoom`
calls `controller.abort()`, the in-flight `fetch` rejects with
`AbortError: signal is aborted without reason`. This is expected and is caught:

```ts
// note-view.tsx
.catch((err) => {
  if (!(err instanceof DOMException && err.name === "AbortError")) {
    console.error("[note-view] fetchTicket error:", err)
  }
  throw err
})
```

The rethrown `AbortError` propagates through `connectRoom`'s async body, which
then catches it and re-throws as `"connection superseded"`. `NoteView`'s outer
catch swallows it silently:

```ts
.catch((err) => {
  if (err instanceof DOMException && err.name === "AbortError") return
  console.error("[note-view] connectRoom failed:", err)
})
```

---

## End-to-End Flow Diagram

```
Browser Tab A                    Notes Service              ScyllaDB
─────────────                    ─────────────              ────────
[NoteView mounts]
    │
    ├─ POST /ws/ticket ─────────► authenticateHTTP
    │   (JWT in header)              │
    │                            IssueWSTicket
    │                                │
    │                            INSERT ws_tickets ────────► ws_tickets
    │                                                        (TTL 120s)
    ◄── { ticket: "abc…" } ─────────┘
    │
    ├─ GET /ws/notes/{id}?ticket=abc…
    │                            RedeemWSTicket
    │                                │
    │                            SELECT + DELETE IF EXISTS ► ws_tickets
    │                                │
    │                            HandleNoteWS
    │                                │
    │                            getOrCreateRoom
    │                                │
    │                            GetYjsState ────────────── ► yjs_state
    │                                │ (hydrate currentValue)
    │                            room.run() started
    │                                │
    ◄── {"type":"init","value":…} ───┤ sendInitialState
    │                                │
[Editor renders initial value]       │

[User types]
    │
    ├─ {"type":"update","value":…} ─►│ room.broadcast chan
    │                                │
    │                            Update currentValue
    │                            Append to buffer
    │                            flushTimer.Reset(400ms)
    │                                │
    │            ┌───────────────────┤ fan-out to other clients
    │            │                   │
    │            ▼                   │
    │        Browser Tab B           │
    │        applyRemoteValue        │
    │                                │
    │                         [400ms later]
    │                            flush()
    │                                │
    │                            BatchAppendNoteUpdates ───► yjs_updates
    │                            UpsertYjsState ───────────► yjs_state

[User closes tab]
    │
    ├─ ws.close(1000) ──────────────►│ readPump error → r.leave <- c
    │                                │
    │                            delete(r.clients, c)
    │                            if empty:
    │                                flush() (final)
    │                                onEmpty() → delete room from Hub
    │                                run() goroutine exits
```
