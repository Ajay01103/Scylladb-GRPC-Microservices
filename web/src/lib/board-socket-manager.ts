/**
 * Generic WebSocket room manager.
 *
 * Works for both whiteboards (/ws/boards/{id}) and notes (/ws/notes/{id}).
 */

export type SocketState = "connecting" | "open" | "closed" | "error"

export interface ConnectOptions {
  /** Stable key for this connection — typically the resource ID (boardId / noteId). */
  roomId: string
  /** WebSocket base URL, e.g. "ws://localhost:9093" */
  wsUrl: string
  /** Path on the WS server, e.g. "/ws/boards/123" or "/ws/notes/456" */
  path: string
  /** Called to fetch a one-time ticket before each (re-)connect attempt. */
  fetchTicket: (signal?: AbortSignal) => Promise<string>
}

interface SocketEntry {
  ws: WebSocket
  opts: ConnectOptions
  retries: number
  retryTimer?: ReturnType<typeof setTimeout>
  token: number
}

interface PendingConnection {
  promise: Promise<WebSocket>
  controller: AbortController
  token: number
}

const sockets = new Map<string, SocketEntry>()
const pendingConnections = new Map<string, PendingConnection>()
const connectionTokens = new Map<string, number>()
const msgListeners = new Map<string, Set<(e: MessageEvent) => void>>()
const stateListeners = new Map<string, Set<(s: SocketState) => void>>()
const sendQueue = new Map<string, (string | ArrayBuffer | ArrayBufferView)[]>()

function notifyState(roomId: string, state: SocketState) {
  stateListeners.get(roomId)?.forEach((fn) => fn(state))
}

function flushQueue(roomId: string, ws: WebSocket) {
  const queue = sendQueue.get(roomId) ?? []
  sendQueue.delete(roomId)
  for (const msg of queue) ws.send(msg as any)
}

function scheduleReconnect(entry: SocketEntry) {
  const currentToken = connectionTokens.get(entry.opts.roomId)
  if (currentToken !== entry.token) return

  const delay = Math.min(500 * 2 ** entry.retries, 30_000)
  entry.retries++
  entry.retryTimer = setTimeout(() => {
    connectRoom(entry.opts).catch(() => {
      // FetchTicket or connection failed — retry with backoff.
      scheduleReconnect(entry)
    })
  }, delay)
}

// ─── Core API ───────────────────────────────────────────────────────────────

export async function connectRoom(opts: ConnectOptions): Promise<WebSocket> {
  const { roomId } = opts

  const existing = sockets.get(roomId)
  if (existing?.ws.readyState === WebSocket.OPEN) return existing.ws

  // Bug-2 fix: only reuse a pending connection if its token still matches the
  // current token. If disconnectRoom bumped the token and then cleared the
  // pending entry, this check is moot (pending will be undefined). But if there
  // is a race where the pending entry wasn't cleared yet, we must not reuse a
  // promise whose ticket was already consumed by LWT delete.
  const currentToken = connectionTokens.get(roomId) ?? 0
  const pending = pendingConnections.get(roomId)
  if (pending && pending.token === currentToken) return pending.promise
  // Stale pending entry (token mismatch) — evict it so it can't be reused.
  if (pending && pending.token !== currentToken) {
    pending.controller.abort()
    pendingConnections.delete(roomId)
  }

  if (existing?.ws.readyState === WebSocket.CONNECTING) {
    return new Promise((resolve, reject) => {
      existing.ws.addEventListener("open", () => resolve(existing.ws), { once: true })
      existing.ws.addEventListener("error", reject, { once: true })
    })
  }

  const token = currentToken + 1
  connectionTokens.set(roomId, token)
  const controller = new AbortController()

  const connection = (async (): Promise<WebSocket> => {
    try {
      notifyState(roomId, "connecting")
      const ticket = await opts.fetchTicket(controller.signal)
      if (controller.signal.aborted || connectionTokens.get(roomId) !== token) {
        throw new DOMException("connection superseded", "AbortError")
      }

      const url = new URL(opts.wsUrl)
      url.pathname = opts.path
      url.searchParams.set("ticket", ticket)

      const ws = new WebSocket(url.toString())
      ws.binaryType = "arraybuffer"
      const entry: SocketEntry = { ws, opts, retries: 0, token }
      sockets.set(roomId, entry)

      ws.addEventListener("open", () => {
        if (connectionTokens.get(roomId) !== token) {
          ws.close(1000, "stale connection")
          return
        }
        entry.retries = 0
        flushQueue(roomId, ws)
        notifyState(roomId, "open")
      })

      ws.addEventListener("message", (e) => {
        msgListeners.get(roomId)?.forEach((fn) => fn(e))
      })

      ws.addEventListener("close", (ev) => {
        if (connectionTokens.get(roomId) !== token) return
        sockets.delete(roomId)
        if (ev.code === 1000) {
          notifyState(roomId, "closed")
          return
        }
        notifyState(roomId, "error")
        scheduleReconnect(entry)
      })

      ws.addEventListener("error", () => notifyState(roomId, "error"))
      return ws
    } catch (error) {
      if (controller.signal.aborted || connectionTokens.get(roomId) !== token) {
        throw new DOMException("connection superseded", "AbortError")
      }
      notifyState(roomId, "error")
      console.error(`[socket-manager] failed to connect room "${roomId}"`, error)
      throw error
    }
  })()

  pendingConnections.set(roomId, { promise: connection, controller, token })
  try {
    return await connection
  } finally {
    const pendingEntry = pendingConnections.get(roomId)
    if (pendingEntry?.promise === connection) {
      pendingConnections.delete(roomId)
    }
  }
}

export function disconnectRoom(roomId: string) {
  connectionTokens.set(roomId, (connectionTokens.get(roomId) ?? 0) + 1)

  const pending = pendingConnections.get(roomId)
  pending?.controller.abort()
  pendingConnections.delete(roomId)

  const entry = sockets.get(roomId)
  if (!entry) return
  clearTimeout(entry.retryTimer)
  entry.ws.close(1000, "client disconnect")
  sockets.delete(roomId)
  sendQueue.delete(roomId)
  notifyState(roomId, "closed")
}

export function sendRoomMessage(roomId: string, data: string | ArrayBuffer | ArrayBufferView) {
  const entry = sockets.get(roomId)
  if (entry?.ws.readyState === WebSocket.OPEN) {
    entry.ws.send(data as any)
    return
  }
  if (!sendQueue.has(roomId)) sendQueue.set(roomId, [])
  sendQueue.get(roomId)!.push(data)
}

export function addMessageListener(roomId: string, fn: (e: MessageEvent) => void): () => void {
  if (!msgListeners.has(roomId)) msgListeners.set(roomId, new Set())
  msgListeners.get(roomId)!.add(fn)
  return () => msgListeners.get(roomId)?.delete(fn)
}

export function addStateListener(roomId: string, fn: (s: SocketState) => void): () => void {
  if (!stateListeners.has(roomId)) stateListeners.set(roomId, new Set())
  stateListeners.get(roomId)!.add(fn)
  return () => stateListeners.get(roomId)?.delete(fn)
}

export function getRoomState(roomId: string): SocketState {
  const entry = sockets.get(roomId)
  if (!entry) return "closed"
  switch (entry.ws.readyState) {
    case WebSocket.CONNECTING:
      return "connecting"
    case WebSocket.OPEN:
      return "open"
    default:
      return "closed"
  }
}
