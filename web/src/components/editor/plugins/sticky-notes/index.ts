import { YooptaPlugin } from "@yoopta/editor"
import { StickyNoteComponent, type StickyNoteColor } from "./sticky-notes-coponent"

/**
 * Yoopta custom plugin — Sticky Note
 *
 * A draggable, position:fixed sticky note block, rendered as a void element.
 * The caption text is stored inside element.props.caption so it survives
 * Yjs serialisation and page refreshes.
 */
export const StickyNotePlugin = new YooptaPlugin({
  type: "StickyNote",

  // ── Elements ────────────────────────────────────────────────────────────────
  elements: {
    "sticky-note": {
      render: StickyNoteComponent,
      // nodeType goes inside props per the Yoopta PluginDefaultProps spec
      props: {
        nodeType: "void" as const,
        xOffset: 100,
        yOffset: 100,
        color: "yellow" as StickyNoteColor,
        caption: "",
      },
    },
  },

  // ── Slash-command menu entry ────────────────────────────────────────────────
  options: {
    display: {
      title: "Sticky Note",
      description: "A draggable sticky note",
      icon: "📝",
    },
    shortcuts: ["sticky", "note"],
  },

  // ── Lifecycle ────────────────────────────────────────────────────────────────
  // beforeCreate must return a SlateElement (the inner Slate node).
  // editor.y() is the factory exposed by Yoopta for exactly this purpose.
  lifecycle: {
    beforeCreate: (editor) => {
      return editor.y("sticky-note", {
        props: {
          nodeType: "void" as const,
          xOffset: 100,
          yOffset: 100,
          color: "yellow" as StickyNoteColor,
          caption: "",
        },
        children: [editor.y.text("")],
      })
    },
  },

  // ── Parsers ──────────────────────────────────────────────────────────────────
  parsers: {
    html: {
      // serialize is a plain function at the html level (not inside deserialize)
      serialize: (element, _text) => {
        const { xOffset, yOffset, color, caption } = element.props as {
          xOffset: number
          yOffset: number
          color: StickyNoteColor
          caption: string
        }
        const bg = color === "pink" ? "#ffb3c1" : "#fff59d"
        return (
          `<div data-sticky-note="true" data-x="${xOffset}" data-y="${yOffset}" data-color="${color}" ` +
          `style="position:fixed;left:${xOffset}px;top:${yOffset}px;background:${bg};` +
          `padding:12px;border-radius:8px;width:200px;box-shadow:2px 3px 8px rgba(0,0,0,.2);">` +
          `${caption}</div>`
        )
      },
      deserialize: {
        nodeNames: ["DIV"],
        parse: (el, editor) => {
          if (el.dataset.stickyNote !== "true") return undefined
          return editor.y("sticky-note", {
            props: {
              nodeType: "void" as const,
              xOffset: Number(el.dataset.x ?? 100),
              yOffset: Number(el.dataset.y ?? 100),
              color: (el.dataset.color as StickyNoteColor) ?? "yellow",
              caption: el.textContent ?? "",
            },
            children: [editor.y.text("")],
          })
        },
      },
    },
    markdown: {
      serialize: (element, _text) => {
        const { color, caption } = element.props as {
          color: StickyNoteColor
          caption: string
        }
        return `> 📝 **Sticky Note** (${color})\n> ${caption}\n`
      },
    },
  },
})
