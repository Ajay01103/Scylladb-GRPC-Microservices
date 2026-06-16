"use client"

import { useEffect, useRef, useState } from "react"

import {
  Tldraw,
  DefaultToolbar,
  DefaultToolbarContent,
  createTLStore,
  defaultHandleExternalTextContent,
  loadSnapshot,
  useEditor,
  type Editor,
  type TLAsset,
} from "tldraw"

import { useAuth } from "@/lib/auth-context"
import { requestWhiteboardWsTicket } from "@/modules/whiteboard/api/ws-ticket"
import { connectRoom, disconnectRoom, addMessageListener, sendRoomMessage } from "@/lib/board-socket-manager"
import { MermaidDialogHost, MermaidToolbarButton } from "./mermaid-button"
import { insertMermaidDiagram } from "./mermaid-utils"

import { uploadAssetAction, downloadAssetAction } from "@/actions/assets"

type WhiteboardCanvasProps = {
  boardId: string
  workspaceId: string
  slug: string
}

const MERMAID_KEYWORD =
  /^\s*(flowchart|graph|sequenceDiagram|stateDiagram|classDiagram|erDiagram|gantt|pie|gitGraph|mindmap)/

async function messageDataToText(data: unknown) {
  if (typeof data === "string") {
    return data
  }

  if (data instanceof ArrayBuffer) {
    return new TextDecoder().decode(data)
  }

  if (data instanceof Blob) {
    return data.text()
  }

  return null
}

function extractDocumentPayload(text: string): unknown | null {
  try {
    const payload = JSON.parse(text) as
      | { type?: string; document?: unknown }
      | Record<string, unknown>

    if (payload && typeof payload === "object") {
      if (Object.prototype.hasOwnProperty.call(payload, "document")) {
        return payload.document ?? null
      }

      if ((payload as { type?: string }).type === "ack") {
        return null
      }

      return payload
    }
  } catch {
    return null
  }

  return null
}

function MermaidPasteHandler() {
  const editor = useEditor()

  useEffect(() => {
    const handleTextContent = async (content: { text: string }) => {
      if (!MERMAID_KEYWORD.test(content.text)) {
        await defaultHandleExternalTextContent(editor, content)
        return
      }

      try {
        await insertMermaidDiagram(editor, content.text)
      } catch {
        await defaultHandleExternalTextContent(editor, content)
      }
    }

    editor.registerExternalContentHandler("text", handleTextContent)

    return () => {
      editor.registerExternalContentHandler("text", null)
    }
  }, [editor])

  return null
}

function WhiteboardToolbar(props: React.ComponentProps<typeof DefaultToolbar>) {
  return (
    <DefaultToolbar {...props}>
      {props.children ?? <DefaultToolbarContent />}
      <MermaidToolbarButton />
    </DefaultToolbar>
  )
}

export function WhiteboardCanvas({ boardId, workspaceId, slug }: WhiteboardCanvasProps) {
  const { accessToken } = useAuth()
  const [editor, setEditor] = useState<Editor | null>(null)
  const editorRef = useRef<Editor | null>(null)
  const pendingDocumentRef = useRef<unknown | null>(null)
  const loadedSnapshotRef = useRef(false)

  // Keep the access token in a ref so the fetchTicket closure always uses the
  // latest token on reconnect attempts, not the stale value from mount time.
  const accessTokenRef = useRef<string | null>(null)
  accessTokenRef.current = accessToken

  // Cache object URLs locally to bypass CORS when rendering images.
  const blobUrlCacheRef = useRef<Map<string, string>>(new Map())
  const pendingFetchesRef = useRef<Set<string>>(new Set())

  // Clean up blob URLs when component unmounts to prevent memory leaks.
  useEffect(() => {
    return () => {
      blobUrlCacheRef.current.forEach((url) => {
        URL.revokeObjectURL(url)
      })
    }
  }, [])

  // The asset store MUST be embedded in createTLStore — passing `assets` as a
  // <Tldraw> prop is silently ignored when you supply your own `store`.
  // Capture boardId/workspaceId in a ref so the store initializer (which runs
  // once) still has fresh values via the closure.
  const boardIdRef = useRef(boardId)
  const workspaceIdRef = useRef(workspaceId)
  useEffect(() => { boardIdRef.current = boardId }, [boardId])
  useEffect(() => { workspaceIdRef.current = workspaceId }, [workspaceId])

  const [store] = useState(() =>
    createTLStore({
      assets: {
        async upload(asset: TLAsset, file: File) {
          const formData = new FormData()
          formData.append("file", file)
          formData.append("boardId", boardIdRef.current)
          formData.append("workspaceId", workspaceIdRef.current)
          formData.append("assetId", asset.id)

          const res = await uploadAssetAction(formData)
          if (!res.success || !res.src) {
            throw new Error(res.error ?? "Failed to upload asset to S3")
          }
          return { src: res.src }
        },
        resolve(asset: TLAsset) {
          const src = asset.props.src
          if (src && typeof src === "string" && (src.startsWith("http") || src.startsWith("/uploads/"))) {
            // Check cache first
            const cachedBlobUrl = blobUrlCacheRef.current.get(asset.id)
            if (cachedBlobUrl) {
              return cachedBlobUrl
            }

            // Check if already fetching
            if (!pendingFetchesRef.current.has(asset.id)) {
              pendingFetchesRef.current.add(asset.id)

              // Extract S3 key from the S3 URL
              const match = src.match(/\/uploads\/(workspaces\/.+)/)
              const s3Key = match ? match[1] : null

              if (s3Key) {
                downloadAssetAction(s3Key).then((res) => {
                  if (res.success && res.base64 && res.mimeType) {
                    const byteCharacters = atob(res.base64)
                    const byteNumbers = new Array(byteCharacters.length)
                    for (let i = 0; i < byteCharacters.length; i++) {
                      byteNumbers[i] = byteCharacters.charCodeAt(i)
                    }
                    const byteArray = new Uint8Array(byteNumbers)
                    const blob = new Blob([byteArray], { type: res.mimeType })
                    const blobUrl = URL.createObjectURL(blob)

                    blobUrlCacheRef.current.set(asset.id, blobUrl)

                    // Update the asset's metadata in the store to trigger a re-resolve & redraw.
                    if (editorRef.current) {
                      editorRef.current.updateAssets([
                        {
                          ...asset,
                          meta: {
                            ...asset.meta,
                            resolvedAt: Date.now(),
                          },
                        },
                      ])
                    }
                  }
                }).catch((err) => {
                  console.error("Failed to load asset via server action:", err)
                })
              }
            }
          }
          return src ?? null
        },
      },
    })
  )

  useEffect(() => {
    editorRef.current = editor
  }, [editor])

  const applySnapshot = (targetEditor: Editor, document: unknown) => {
    if (!document) {
      return
    }

    loadSnapshot(targetEditor.store, { document: document as never })
    loadedSnapshotRef.current = true
    pendingDocumentRef.current = null
  }

  useEffect(() => {
    if (!accessToken) return

    const fetchTicket = async (signal?: AbortSignal) => {
      const token = accessTokenRef.current
      if (!token) throw new Error("no access token")
      return await requestWhiteboardWsTicket(token, signal)
    }

    connectRoom({
      roomId: boardId,
      wsUrl: process.env.NEXT_PUBLIC_WHITEBOARD_WS_URL ?? "ws://localhost:9093",
      path: `/ws/boards/${boardId}`,
      fetchTicket,
    }).catch(() => {})

    return () => {
      disconnectRoom(boardId)
    }
  }, [accessToken, boardId])

  useEffect(() => {
    const unsub = addMessageListener(boardId, async (e) => {
      const text = await messageDataToText(e.data)
      if (!text) return
      const document = extractDocumentPayload(text)
      if (!document) return

      const currentEditor = editorRef.current
      if (currentEditor) applySnapshot(currentEditor, document)
      else pendingDocumentRef.current = document
    })

    return unsub
  }, [boardId])

  useEffect(() => {
    if (!editor) {
      return
    }

    if (!loadedSnapshotRef.current && pendingDocumentRef.current) {
      applySnapshot(editor, pendingDocumentRef.current)
    }

    let debounceTimer: ReturnType<typeof setTimeout> | null = null

    const sendOp = async () => {
      const snapshot = editor.getSnapshot()
      const msg = JSON.stringify({
        type: "snapshot",
        clock: Date.now(),
        document: snapshot.document,
      })
      sendRoomMessage(boardId, msg)
    }

    const unsubscribe = editor.store.listen(
      () => {
        if (debounceTimer) clearTimeout(debounceTimer)

        debounceTimer = setTimeout(() => {
          void sendOp()
          debounceTimer = null
        }, 200)
      },
      { source: "user", scope: "document" },
    )

    return () => {
      if (debounceTimer) clearTimeout(debounceTimer)
      unsubscribe()
    }
  }, [editor])

  return (
    <div
      className="h-svh w-full overflow-hidden bg-background text-foreground"
      data-whiteboard-slug={slug}>
      <div className="h-full w-full">
        <Tldraw
          onMount={(mountedEditor) => {
            setEditor(mountedEditor)
          }}
          components={{ Toolbar: WhiteboardToolbar }}
          store={store}>
          <MermaidPasteHandler />
        </Tldraw>
      </div>
      <MermaidDialogHost editor={editor} />
    </div>
  )
}
