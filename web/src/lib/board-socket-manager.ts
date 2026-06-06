const WS_BASE = process.env.NEXT_PUBLIC_WHITEBOARD_WS_URL ?? "ws://localhost:9093"

export type SocketState = "connecting" | "open" | "closed" | "error"

interface SocketEntry {
  ws: WebSocket
  boardId: string
  retries: number
  retryTimer?: ReturnType<typeof setTimeout>
  fetchTicket: (signal?: AbortSignal) => Promise<string>
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
const sendQueue = new Map<string, (string | ArrayBuffer)[]>()

function notifyState(boardId: string, state: SocketState) {
  stateListeners.get(boardId)?.forEach((fn) => fn(state))
}

function flushQueue(boardId: string, ws: WebSocket) {
  const queue = sendQueue.get(boardId) ?? []
  sendQueue.delete(boardId)
  for (const msg of queue) ws.send(msg)
}

function scheduleReconnect(entry: SocketEntry) {
  const currentToken = connectionTokens.get(entry.boardId)
  if (currentToken !== entry.token) {
    return
  }

  const delay = Math.min(500 * 2 ** entry.retries, 30_000)
  entry.retries++
  entry.retryTimer = setTimeout(() => {
    void connectBoard(entry.boardId, entry.fetchTicket)
  }, delay)
}

export async function connectBoard(
  boardId: string,
  fetchTicket: (signal?: AbortSignal) => Promise<string>,
): Promise<WebSocket> {
  const existing = sockets.get(boardId)
  if (existing?.ws.readyState === WebSocket.OPEN) return existing.ws

  const pending = pendingConnections.get(boardId)
  if (pending) return pending.promise

  if (existing?.ws.readyState === WebSocket.CONNECTING) {
    return new Promise((resolve, reject) => {
      existing.ws.addEventListener("open", () => resolve(existing.ws), { once: true })
      existing.ws.addEventListener("error", reject, { once: true })
    })
  }

  const token = (connectionTokens.get(boardId) ?? 0) + 1
  connectionTokens.set(boardId, token)
  const controller = new AbortController()

  const connection = (async () => {
    try {
      notifyState(boardId, "connecting")
      const ticket = await fetchTicket(controller.signal)
      if (controller.signal.aborted || connectionTokens.get(boardId) !== token) {
        return new Promise<WebSocket>(() => {})
      }

      const url = new URL(WS_BASE)
      url.pathname = `/ws/boards/${boardId}`
      url.searchParams.set("ticket", ticket)

      const ws = new WebSocket(url.toString())
      ws.binaryType = "arraybuffer"
      const entry: SocketEntry = { ws, boardId, retries: 0, fetchTicket, token }
      sockets.set(boardId, entry)

      ws.addEventListener("open", () => {
        if (connectionTokens.get(boardId) !== token) {
          ws.close(1000, "stale connection")
          return
        }
        entry.retries = 0
        flushQueue(boardId, ws)
        notifyState(boardId, "open")
      })

      ws.addEventListener("message", (e) => {
        // Dispatch to listeners
        msgListeners.get(boardId)?.forEach((fn) => fn(e))
      })

      ws.addEventListener("close", (ev) => {
        if (connectionTokens.get(boardId) !== token) {
          return
        }
        sockets.delete(boardId)
        if (ev.code === 1000) {
          notifyState(boardId, "closed")
          return
        }
        notifyState(boardId, "error")
        scheduleReconnect(entry)
      })

      ws.addEventListener("error", () => notifyState(boardId, "error"))
      return ws
    } catch (error) {
      if (controller.signal.aborted || connectionTokens.get(boardId) !== token) {
        return new Promise<WebSocket>(() => {})
      }

      notifyState(boardId, "error")
      console.error("failed to connect whiteboard websocket", error)
      return new Promise<WebSocket>(() => {})
    }
  })()

  pendingConnections.set(boardId, { promise: connection, controller, token })
  try {
    return await connection
  } finally {
    const pendingEntry = pendingConnections.get(boardId)
    if (pendingEntry?.promise === connection) {
      pendingConnections.delete(boardId)
    }
  }
}

export function disconnectBoard(boardId: string) {
  connectionTokens.set(boardId, (connectionTokens.get(boardId) ?? 0) + 1)

  const pending = pendingConnections.get(boardId)
  pending?.controller.abort()
  pendingConnections.delete(boardId)

  const entry = sockets.get(boardId)
  if (!entry) return
  clearTimeout(entry.retryTimer)
  entry.ws.close(1000, "client disconnect")
  sockets.delete(boardId)
  sendQueue.delete(boardId)
  notifyState(boardId, "closed")
}

export function sendBoardMessage(boardId: string, data: string | ArrayBuffer) {
  const entry = sockets.get(boardId)
  if (entry?.ws.readyState === WebSocket.OPEN) {
    entry.ws.send(data)
    return
  }
  if (!sendQueue.has(boardId)) sendQueue.set(boardId, [])
  sendQueue.get(boardId)!.push(data)
}

export function addMessageListener(boardId: string, fn: (e: MessageEvent) => void): () => void {
  if (!msgListeners.has(boardId)) msgListeners.set(boardId, new Set())
  msgListeners.get(boardId)!.add(fn)
  return () => msgListeners.get(boardId)?.delete(fn)
}

export function addStateListener(boardId: string, fn: (s: SocketState) => void): () => void {
  if (!stateListeners.has(boardId)) stateListeners.set(boardId, new Set())
  stateListeners.get(boardId)!.add(fn)
  return () => stateListeners.get(boardId)?.delete(fn)
}

export function getSocketState(boardId: string): SocketState {
  const entry = sockets.get(boardId)
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

export default {
  connectBoard,
  disconnectBoard,
  sendBoardMessage,
  addMessageListener,
  addStateListener,
}
