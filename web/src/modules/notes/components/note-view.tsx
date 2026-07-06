"use client"

import { useCallback, useEffect, useRef, useState } from "react"
import { Loader2 } from "lucide-react"
import { type YooEditor, type YooptaContentValue } from "@yoopta/editor"

import { tokenStore } from "@/lib/token-store"
import {
  connectRoom,
  disconnectRoom,
  sendRoomMessage,
  addMessageListener,
  addStateListener,
  type SocketState,
} from "@/lib/board-socket-manager"
import { requestNotesWsTicket } from "@/modules/notes/api/ws-ticket"
import { useCurrentUser } from "@/modules/auth/api/use-current-user"
import { useNoteBySlug } from "@/modules/notes/api/use-notes"
import {
  setNoteAssetContext,
  clearNoteAssetContext,
} from "@/modules/notes/api/note-asset-context"
import { FullSetupEditor } from "@/components/editor"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { AvatarStack } from "@/components/ui/avatar-stack"
import { Cursor, CursorPointer, CursorBody, CursorName } from "@/components/ui/cursor"

const NOTES_WS_URL = process.env.NEXT_PUBLIC_NOTES_WS_URL ?? "ws://localhost:9092"

// ── Wire message types ────────────────────────────────────────────────────────

type YooptaBlock = YooptaContentValue[string]

/**
 * Block-level patch sent on every keystroke instead of the full document.
 * upserted: blocks that were added or changed (keyed by block UUID)
 * deleted:  block UUIDs that were removed
 */
type PatchMessage = {
  type: "patch"
  upserted: Record<string, YooptaBlock>
  deleted: string[]
}

type InitMessage = { type: "init"; value?: YooptaContentValue | null }

type PresenceListMessage = {
  type: "presence_list"
  users: Array<{ userId: string; name: string; color: string; avatar?: string }>
}

type PresenceMessage = {
  type: "presence"
  userId: string
  name: string
  color: string
  avatar?: string
  joined: boolean
}

type CursorMessage = {
  type: "cursor"
  userId: string
  x: number
  y: number
}

type IncomingMessage =
  | InitMessage
  | PatchMessage
  | PresenceListMessage
  | PresenceMessage
  | CursorMessage

type UserInfo = { userId: string; name: string; color: string; avatar?: string }
type CursorInfo = { x: number; y: number; name: string; color: string }

/**
 * Compute a block-level diff between two Yoopta document snapshots.
 * Only the changed / added / removed blocks are included — not the whole doc.
 */
function diffBlocks(prev: YooptaContentValue, next: YooptaContentValue): PatchMessage {
  const upserted: Record<string, YooptaBlock> = {}
  const deleted: string[] = []

  for (const id of Object.keys(next)) {
    if (JSON.stringify(prev[id]) !== JSON.stringify(next[id])) {
      upserted[id] = next[id]
    }
  }
  for (const id of Object.keys(prev)) {
    if (!next[id]) deleted.push(id)
  }

  return { type: "patch", upserted, deleted }
}

/**
 * Apply a received patch on top of the local document snapshot.
 * Returns the merged document — does not mutate the input.
 */
function applyPatch(local: YooptaContentValue, patch: PatchMessage): YooptaContentValue {
  const next = { ...local, ...patch.upserted }
  for (const id of patch.deleted) delete next[id]
  return next
}

function hashColor(str: string): string {
  let hash = 0
  for (let i = 0; i < str.length; i++) {
    hash = str.charCodeAt(i) + ((hash << 5) - hash)
  }
  const c = (hash & 0x00ffffff).toString(16).padStart(6, "0")
  return `#${c}`
}

type NoteViewProps = { slug: string }

export function NoteView({ slug }: NoteViewProps) {
  const { note, workspaceId, isLoading: isLoadingNote, isError } = useNoteBySlug(slug)

  // Stable ref to the editor — never replaced after first assignment.
  const editorRef = useRef<YooEditor | null>(null)

  // Bug-4: track whether the editor has mounted so the message listener
  // knows whether to apply immediately or queue for later.
  const editorReadyRef = useRef(false)

  // Bug-1: use a ref counter instead of a boolean flag.
  // Slate internally schedules some onChange propagation via setTimeout(fn, 0).
  // A microtask (Promise.resolve().then) fires *before* those macrotasks, so a
  // boolean flag reset in a microtask allows Slate's own cascaded onChange
  // calls to slip through the echo guard. An integer counter decremented in a
  // setTimeout(fn, 50) outlasts all of Slate's queued callbacks.
  const applyingRemote = useRef(0)

  // Serialised snapshot of the last remote value we applied, stored as the
  // raw wire string (not re-serialised from editor.getEditorValue()).
  // Bug-3: Yoopta normalises the value after setEditorValue — reading back
  // with getEditorValue() produces a different string, breaking the echo guard.
  // Storing the wire string and comparing against JSON.stringify(currentValue)
  // keeps both sides normalised the same way.
  const lastRemoteSnapshotRef = useRef<string>("")

  // Bug-4: queue holds full IncomingMessage objects (init or patch), not just
  // values. Messages that arrive before the editor mounts are held here and
  // applied in order once handleEditorReady fires.
  const pendingMessages = useRef<IncomingMessage[]>([])

  // noteId as a ref so the WS callbacks always see the latest value without
  // being listed as useEffect / useCallback deps.
  const { data: currentUser } = useCurrentUser()
  const userRef = useRef(currentUser)
  userRef.current = currentUser

  const lastCursorSendRef = useRef(0)
  const editorContainerRef = useRef<HTMLDivElement>(null)

  const noteIdRef = useRef<string | undefined>(undefined)
  noteIdRef.current = note?.id

  useEffect(() => {
    if (note?.id && workspaceId) {
      setNoteAssetContext(note.id, workspaceId)
    }
    return () => clearNoteAssetContext()
  }, [note?.id, workspaceId])

  // The last document snapshot we sent over the wire. Diffed against the next
  // onChange value to produce a block-level patch instead of a full snapshot.
  const prevValueRef = useRef<YooptaContentValue>({})

  const [socketState, setSocketState] = useState<SocketState>("closed")
  const [activeUsers, setActiveUsers] = useState<Record<string, UserInfo>>({})
  const [cursors, setCursors] = useState<Record<string, CursorInfo>>({})

  const activeUsersRef = useRef(activeUsers)
  activeUsersRef.current = activeUsers

  // ── Push a remote Yoopta value into the editor ─────────────────────────────
  const applyRemoteValue = useCallback((value: YooptaContentValue) => {
    if (!value || Object.keys(value).length === 0) return

    const editor = editorRef.current
    if (!editor) {
      // Should not reach here after bug-4 fix (editorReadyRef guards the call
      // sites), but keep as a safety net.
      return
    }

    // Bug-1: increment the counter before any async work.
    applyingRemote.current += 1

    const currentValue = editor.getEditorValue()
    const currentIds = Object.keys(currentValue)
    const newIds = Object.keys(value)

    // Bug-2: include block-type changes in the structural check.
    // A type change (paragraph → heading) keeps the same block count and IDs
    // but the Slate editor for that block still holds the old schema. Taking
    // the text-only path would silently discard the type change.
    const hasStructuralChange =
      currentIds.length !== newIds.length ||
      currentIds.some((id) => !value[id]) ||
      newIds.some((id) => !currentValue[id]) ||
      newIds.some((id) => currentValue[id]?.type !== value[id]?.type) ||
      newIds.some(
        (id) =>
          JSON.stringify(currentValue[id]?.meta) !== JSON.stringify(value[id]?.meta),
      )

    if (hasStructuralChange) {
      editor.setEditorValue(value)
    } else {
      // Text-only fast path: push changes directly into each Slate editor
      // instance so Slate v0.71+'s "value is initial-only" constraint is
      // bypassed — setEditorValue won't update existing instances.
      const blockEditors =
        (editor as any).blockEditorsMap ?? (editor as any).editorBlocksMap

      for (const [blockId, newBlock] of Object.entries(value)) {
        const oldBlock = currentValue[blockId]
        if (!oldBlock) continue
        if (JSON.stringify(oldBlock.value) === JSON.stringify(newBlock.value)) continue

        const slateEd = blockEditors?.[blockId]
        if (!slateEd) {
          // Fallback: missing Slate editor instance — use setEditorValue for safety.
          editor.setEditorValue(value)
          break
        }

        slateEd.children = newBlock.value
        slateEd.onChange()
      }
    }

    // Bug-3: store the received wire string, not a re-read from the editor.
    // editor.getEditorValue() returns a Yoopta-normalised form that may differ
    // from the incoming value (key ordering, added defaults, etc.), which would
    // break the echo guard in onChange.
    lastRemoteSnapshotRef.current = JSON.stringify(value)
    // Use the incoming value directly as the diff baseline too — same reason.
    prevValueRef.current = value

    // Bug-1: decrement the counter in a macrotask so we outlast Slate's own
    // internal setTimeout(fn, 0) onChange callbacks, which fire after any
    // microtask (Promise.resolve().then). 50 ms is well above Slate's 0 ms
    // and low enough that the user never perceives the suppression window.
    setTimeout(() => {
      applyingRemote.current -= 1
    }, 50)
  }, [])

  // ── WebSocket connection + lifecycle (single effect like whiteboard) ──────
  useEffect(() => {
    if (!note?.id) return
    const noteId = note.id

    // Reset the diff baseline when connecting to a new note so stale state
    // from a previous note (or previous mount) doesn't pollute patch merging.
    prevValueRef.current = {}
    lastRemoteSnapshotRef.current = ""

    connectRoom({
      roomId: noteId,
      wsUrl: NOTES_WS_URL,
      path: `/ws/notes/${noteId}`,
      // Bug-1 fix: never close over a token value. Call ensureValidAccessToken()
      // on every attempt so silent JWT refreshes are picked up automatically.
      // scheduleReconnect will therefore always use a fresh, non-expired token
      // even when the original JWT has long since expired.
      fetchTicket: async (signal) => {
        const token = await tokenStore.ensureValidAccessToken()
        if (!token) {
          throw new Error("[note-view] fetchTicket: no access token")
        }
        return requestNotesWsTicket(token, signal).catch((err) => {
          if (!(err instanceof DOMException && err.name === "AbortError")) {
            console.error("[note-view] fetchTicket error:", err)
          }
          throw err
        })
      },
    }).catch((err) => {
      if (err instanceof DOMException && err.name === "AbortError") return
      console.error("[note-view] connectRoom failed:", err)
    })

    return () => {
      disconnectRoom(noteId)
    }
  }, [note?.id])

  // ── Throttled cursor send on mouse move within editor area ────────────────
  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      const noteId = noteIdRef.current
      if (!noteId) return
      const now = performance.now()
      if (now - lastCursorSendRef.current < 50) return
      lastCursorSendRef.current = now
      const el = editorContainerRef.current
      if (!el) return
      const rect = el.getBoundingClientRect()
      sendRoomMessage(
        noteId,
        JSON.stringify({ type: "cursor", x: e.clientX - rect.left, y: e.clientY - rect.top }),
      )
    }

    window.addEventListener("mousemove", handleMouseMove)
    return () => window.removeEventListener("mousemove", handleMouseMove)
  }, [])

  // ── Route a decoded incoming message to applyRemoteValue ─────────────────
  // Extracted so both the live listener and the queue drain share the same
  // logic path (Bug-4).
  const applyIncomingMessage = useCallback(
    (msg: IncomingMessage) => {
      if (msg.type === "init" && msg.value) {
        // Full snapshot on join — reset the diff baseline to the full document.
        // This is the authoritative state; all subsequent patches are relative to it.
        prevValueRef.current = msg.value
        lastRemoteSnapshotRef.current = JSON.stringify(msg.value)
        applyRemoteValue(msg.value)
      } else if (msg.type === "patch") {
        // Merge patch onto the current local snapshot.
        // prevValueRef MUST already be populated from the init message before
        // any patch arrives — the server sends init before forwarding patches
        // from peers. If prevValueRef is still empty (race), merging onto {}
        // would produce a document with only the patched blocks, losing all
        // prior content. Guard against this: skip the patch if we don't have
        // a baseline yet (the init is still in-flight or queued).
        if (Object.keys(prevValueRef.current).length === 0) {
          // No baseline yet — queue the patch so it can be replayed after
          // the init arrives and sets prevValueRef.
          pendingMessages.current.push(msg)
          return
        }
        const merged = applyPatch(prevValueRef.current, msg)
        prevValueRef.current = merged
        applyRemoteValue(merged)
      }
    },
    [applyRemoteValue],
  )

  // ── WebSocket message + state listeners (separate, stable lifecycle) ──────
  useEffect(() => {
    if (!note?.id) return
    const noteId = note.id

    const removeMsg = addMessageListener(noteId, (event: MessageEvent) => {
      try {
        let raw: unknown = event.data

        if (raw instanceof ArrayBuffer) {
          const text = new TextDecoder().decode(raw)
          if (text.trimStart().startsWith("{")) raw = text
        }

        if (typeof raw !== "string") return

        const msg = JSON.parse(raw) as IncomingMessage

        if (msg.type === "presence_list") {
          if (!Array.isArray(msg.users)) {
            msg.users = []
          }
          const next: Record<string, UserInfo> = {}
          for (const u of msg.users) {
            if (!u?.userId) continue
            next[u.userId] = {
              userId: u.userId,
              name: u.name ?? "Unknown",
              color: u.color ?? "#6366f1",
              avatar: u.avatar,
            }
          }
          setActiveUsers(next)
          return
        }

        if (msg.type === "presence") {
          if (!msg.userId) return
          if (msg.joined) {
            setActiveUsers((prev) => ({
              ...prev,
              [msg.userId]: {
                userId: msg.userId,
                name: msg.name,
                color: msg.color,
                avatar: msg.avatar,
              },
            }))
          } else {
            setActiveUsers((prev) => {
              const next = { ...prev }
              delete next[msg.userId]
              return next
            })
            setCursors((prev) => {
              const next = { ...prev }
              delete next[msg.userId]
              return next
            })
          }
          return
        }

        if (msg.type === "cursor") {
          if (msg.userId == null || msg.x == null || msg.y == null) return
          const user = activeUsersRef.current[msg.userId]
          setCursors((prev) => ({
            ...prev,
            [msg.userId]: {
              x: msg.x,
              y: msg.y,
              name: user?.name ?? msg.userId,
              color: user?.color ?? "#6366f1",
            },
          }))
          return
        }

        // Bug-4: if the editor hasn't mounted yet, queue the full message
        // (not just the value) so nothing is discarded and order is preserved.
        if (!editorReadyRef.current) {
          pendingMessages.current.push(msg)
          return
        }

        applyIncomingMessage(msg)
      } catch (err) {
        console.error("[note-view] ws message error", err)
      }
    })

    const removeState = addStateListener(noteId, (state) => {
      setSocketState(state)
      if (state === "open") {
        const id = noteIdRef.current
        const u = userRef.current
        if (!id) return
        sendRoomMessage(
          id,
          JSON.stringify({
            type: "identify",
            name: u?.name ?? u?.email ?? "User",
            color: u ? hashColor(u.userId) : "#6366f1",
          }),
        )
      }
    })

    return () => {
      removeMsg()
      removeState()
    }
  }, [note?.id, applyRemoteValue])

  // ── Directly subscribe to editor changes ──────────────────────────────
  // Bypasses YooptaEditor's onChange prop which doesn't fire for user edits.
  const changeSubRef = useRef<{
    editor: YooEditor
    handler: (payload: any) => void
  } | null>(null)
  const handleEditorReady = useCallback(
    (editor: YooEditor | null) => {
      // Unsubscribe previous handler (StrictMode double-mount safety).
      const prev = changeSubRef.current
      if (prev) {
        prev.editor.off("change", prev.handler)
        changeSubRef.current = null
      }

      editorRef.current = editor

      if (!editor) {
        // Editor unmounted — mark not ready so incoming messages queue again.
        editorReadyRef.current = false
        return
      }

      const onChange = (value: YooptaContentValue) => {
        // Bug-1: check counter > 0 (not boolean) so Slate's deferred onChange
        // callbacks are still suppressed after the initial microtask would have
        // cleared a boolean flag.
        if (applyingRemote.current > 0) return

        // Bug-3: compare against the stored wire string, not a re-read snapshot.
        if (
          lastRemoteSnapshotRef.current &&
          JSON.stringify(value) === lastRemoteSnapshotRef.current
        ) {
          return
        }

        const id = noteIdRef.current
        if (!id) return

        const patch = diffBlocks(prevValueRef.current, value)
        if (Object.keys(patch.upserted).length === 0 && patch.deleted.length === 0) return

        prevValueRef.current = value
        sendRoomMessage(id, JSON.stringify(patch))
      }

      const handler = (payload: any) => {
        if (!payload?.value) return
        onChange(payload.value)
      }

      editor.on("change", handler)
      changeSubRef.current = { editor, handler }

      // Bug-4: mark the editor ready and drain the full ordered queue.
      // Apply every queued message in arrival order — not just the last one.
      // Intermediate structural changes (add/delete block) must not be skipped.
      editorReadyRef.current = true
      const queue = pendingMessages.current.splice(0)
      for (const msg of queue) {
        applyIncomingMessage(msg)
      }
    },
    [applyIncomingMessage],
  ) // applyIncomingMessage is stable (depends only on applyRemoteValue which is also stable)

  const isConnecting = socketState === "connecting"
  const isErr = socketState === "error"

  if (isLoadingNote) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center text-sm text-muted-foreground">
        <div className="flex items-center gap-2">
          <Loader2 className="size-4 animate-spin" />
          Loading note…
        </div>
      </div>
    )
  }

  if (isError || !note) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center text-sm text-muted-foreground">
        Note not found.
      </div>
    )
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {/* ── Header ────────────────────────────────────────────────────────── */}
      <header className="flex shrink-0 items-center justify-between border-b border-border/60 px-6 py-3">
        <h1 className="truncate text-base font-semibold text-foreground">
          {note.title ?? "Untitled note"}
        </h1>

        <div className="flex items-center gap-3">
          <AvatarStack size={28}>
            {Object.values(activeUsers).map((u) => (
              <Avatar key={u.userId} style={{ borderColor: u.color }} className="border-2">
                {u.avatar ? (
                  <AvatarImage src={u.avatar} alt={u.name} />
                ) : (
                  <AvatarFallback style={{ backgroundColor: u.color, color: "#fff" }}>
                    {u.name.charAt(0).toUpperCase()}
                  </AvatarFallback>
                )}
              </Avatar>
            ))}
          </AvatarStack>

          <span
            className={[
              "inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium",
              socketState === "open"
                ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
                : isConnecting
                  ? "bg-amber-500/10 text-amber-600 dark:text-amber-400"
                  : isErr
                    ? "bg-rose-500/10 text-rose-600 dark:text-rose-400"
                    : "bg-muted text-muted-foreground",
            ].join(" ")}>
            <span
              className={[
                "size-1.5 rounded-full",
                socketState === "open"
                  ? "animate-pulse bg-emerald-500"
                  : isConnecting
                    ? "animate-ping bg-amber-500"
                    : isErr
                      ? "bg-rose-500"
                      : "bg-muted-foreground",
              ].join(" ")}
            />
            {socketState === "open"
              ? "Live"
              : isConnecting
                ? "Connecting…"
                : isErr
                  ? "Disconnected"
                  : "Offline"}
          </span>
        </div>
      </header>

      {/* ── Editor ────────────────────────────────────────────────────────── */}
      <div
        ref={editorContainerRef}
        className="relative min-h-0 flex-1 overflow-y-auto px-4 py-8 sm:px-8 lg:px-16">
        <FullSetupEditor
          editorRef={editorRef}
          onEditorReady={handleEditorReady}
        />

        {/* Remote cursors */}
        {Object.entries(cursors).map(([userId, cur]) => (
          <Cursor
            key={userId}
            className="absolute z-50 transition-transform duration-100 ease-out"
            style={{ left: cur.x, top: cur.y, color: cur.color }}
          >
            <CursorPointer />
            <CursorBody>
              <CursorName>{cur.name}</CursorName>
            </CursorBody>
          </Cursor>
        ))}
      </div>
    </div>
  )
}
