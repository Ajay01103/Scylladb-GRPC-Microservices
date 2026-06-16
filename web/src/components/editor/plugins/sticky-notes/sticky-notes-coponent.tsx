"use client"

import { useEffect, useRef, useState } from "react"
import { createPortal } from "react-dom"
import { Elements, useYooptaEditor, type PluginElementRenderProps } from "@yoopta/editor"

export type StickyNoteColor = "pink" | "yellow"

export type StickyNoteProps = {
  nodeType: "void"
  xOffset: number
  yOffset: number
  color: StickyNoteColor
  caption: string
}

// Background colours per note colour
const BG: Record<StickyNoteColor, string> = {
  pink: "#ffb3c1",
  yellow: "#fbec67ff",
}
// Slightly darker shade for the textarea
const BG_DARK: Record<StickyNoteColor, string> = {
  pink: "#ffa0b5",
  yellow: "#f4eb9cff",
}

// ─── Main element component ───────────────────────────────────────────────────

export function StickyNoteComponent({
  attributes,
  children,
  element,
  blockId,
}: PluginElementRenderProps) {
  const editor = useYooptaEditor()
  const { xOffset, yOffset, color, caption } = element.props as StickyNoteProps

  // Local position — initialised from persisted props
  const [pos, setPos] = useState({ x: xOffset, y: yOffset })
  // Keep local caption state for snappy typing, flush to Yoopta on blur
  const [text, setText] = useState(caption ?? "")
  const [isVisible, setIsVisible] = useState(true)

  // Sync if external props change (e.g. from Yjs remote update)
  useEffect(() => {
    setPos({ x: xOffset, y: yOffset })
  }, [xOffset, yOffset])
  useEffect(() => {
    setText(caption ?? "")
  }, [caption])

  // ── Pointer-based drag (reliable cross-browser free-drag) ──────────────────
  const isDragging = useRef(false)
  // Offset of the pointer relative to the card's top-left on mousedown
  const dragOffset = useRef({ dx: 0, dy: 0 })
  const cardRef = useRef<HTMLDivElement>(null)

  const handlePointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    // Only drag on the card itself, not on the textarea or buttons
    if ((e.target as HTMLElement).closest("textarea, button")) return
    e.preventDefault()
    isDragging.current = true
    const rect = cardRef.current!.getBoundingClientRect()
    dragOffset.current = { dx: e.clientX - rect.left, dy: e.clientY - rect.top }
    cardRef.current!.setPointerCapture(e.pointerId)
  }

  const handlePointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!isDragging.current) return
    setPos({
      x: e.clientX - dragOffset.current.dx,
      y: e.clientY - dragOffset.current.dy,
    })
  }

  const handlePointerUp = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!isDragging.current) return
    isDragging.current = false
    const newX = e.clientX - dragOffset.current.dx
    const newY = e.clientY - dragOffset.current.dy
    setPos({ x: newX, y: newY })
    // Persist final position into Yoopta / Yjs
    Elements.updateElement(editor, {
      blockId,
      type: "sticky-note",
      props: { xOffset: newX, yOffset: newY },
    })
  }

  // ── Colour toggle ──────────────────────────────────────────────────────────
  const handleToggleColor = (e: React.MouseEvent) => {
    e.stopPropagation()
    const next: StickyNoteColor = color === "pink" ? "yellow" : "pink"
    Elements.updateElement(editor, {
      blockId,
      type: "sticky-note",
      props: { color: next },
    })
  }

  // ── Close / delete ─────────────────────────────────────────────────────────
  const handleClose = (e: React.MouseEvent) => {
    e.stopPropagation()
    setIsVisible(false)
    editor.deleteBlock({ blockId })
  }

  // ── Caption persistence ────────────────────────────────────────────────────
  const handleCaptionBlur = () => {
    Elements.updateElement(editor, {
      blockId,
      type: "sticky-note",
      props: { caption: text },
    })
  }

  // ─────────────────────────────────────────────────────────────────────────
  // The block anchor sits inline in the document so Yoopta/Slate can track it.
  // The actual floating card is portalled to <body> so it is never clipped by
  // any overflow:hidden ancestor and truly floats over everything.
  // ─────────────────────────────────────────────────────────────────────────
  const card =
    isVisible && typeof document !== "undefined"
      ? createPortal(
          <div
            ref={cardRef}
            contentEditable={false}
            onPointerDown={handlePointerDown}
            onPointerMove={handlePointerMove}
            onPointerUp={handlePointerUp}
            style={{
              position: "fixed",
              left: pos.x,
              top: pos.y,
              width: 220,
              minHeight: 140,
              backgroundColor: BG[color] ?? BG.yellow,
              borderRadius: 10,
              padding: "10px 12px 12px",
              boxShadow: "2px 4px 12px rgba(0,0,0,0.22)",
              zIndex: 9999,
              userSelect: "none",
              touchAction: "none",
              display: "flex",
              flexDirection: "column",
              gap: 6,
              fontFamily: "inherit",
            }}
          >
            {/* ── Header row ── */}
            <div
              style={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                cursor: "grab",
                marginBottom: 2,
              }}
            >
              {/* Drag handle label */}
              <span style={{ fontSize: 11, opacity: 0.85, letterSpacing: "0.04em" }}>
                📝 Sticky Note
              </span>

              <div style={{ display: "flex", gap: 4 }}>
                {/* Colour toggle */}
                <button
                  type="button"
                  onClick={handleToggleColor}
                  onPointerDown={(e) => e.stopPropagation()}
                  title="Toggle colour"
                  aria-label="Toggle sticky note colour"
                  style={buttonStyle}
                >
                  🎨
                </button>
                {/* Close */}
                <button
                  type="button"
                  onClick={handleClose}
                  onPointerDown={(e) => e.stopPropagation()}
                  title="Delete note"
                  aria-label="Delete sticky note"
                  style={{ ...buttonStyle, fontSize: 14 }}
                >
                  ✕
                </button>
              </div>
            </div>

            {/* ── Textarea body ── */}
            <textarea
              value={text}
              onChange={(e) => setText(e.target.value)}
              onBlur={handleCaptionBlur}
              onPointerDown={(e) => e.stopPropagation()}
              placeholder="Type a note…"
              aria-label="Sticky note content"
              rows={5}
              style={{
                flex: 1,
                width: "100%",
                background: BG_DARK[color] ?? BG_DARK.yellow,
                border: "none",
                outline: "none",
                resize: "none",
                fontSize: 13,
                lineHeight: 1.55,
                borderRadius: 6,
                padding: "6px 8px",
                cursor: "text",
                color: "#2a2a2a",
                fontFamily: "inherit",
                userSelect: "text",
              }}
            />
          </div>,
          document.body,
        )
      : null

  return (
    // Slate requires attributes on the outermost rendered DOM node.
    // This anchor is intentionally visible — it shows a small inline pill
    // inside the editor so the user can see "there is a sticky note here"
    // and click it to bring the floating card back if it drifted off-screen.
    <div {...attributes}>
      {/* Hidden Slate cursor placeholder — required for void elements */}
      <span style={{ display: "none" }}>{children}</span>

      {/* Inline anchor — shows inside the document flow */}
      <div
        contentEditable={false}
        onClick={() => setIsVisible((v) => !v)}
        style={{
          display: "inline-flex",
          alignItems: "center",
          gap: 6,
          padding: "3px 10px 3px 6px",
          borderRadius: 999,
          backgroundColor: BG[color] ?? BG.yellow,
          border: "1px solid rgba(0,0,0,0.12)",
          cursor: "pointer",
          fontSize: 12,
          fontWeight: 500,
          color: "#333",
          userSelect: "none",
          boxShadow: "0 1px 3px rgba(0,0,0,0.1)",
        }}
        title={isVisible ? "Hide sticky note" : "Show sticky note"}
        aria-label={isVisible ? "Hide sticky note" : "Show sticky note"}
      >
        📝
        <span style={{ maxWidth: 120, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
          {text || "Sticky Note"}
        </span>
      </div>

      {/* Portal — the actual floating card rendered over everything */}
      {card}
    </div>
  )
}

const buttonStyle: React.CSSProperties = {
  background: "none",
  border: "none",
  cursor: "pointer",
  fontSize: 16,
  lineHeight: 1,
  padding: "2px 3px",
  borderRadius: 4,
  opacity: 0.7,
}
