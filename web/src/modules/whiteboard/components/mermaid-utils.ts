import type { Editor } from "tldraw"

export async function insertMermaidDiagram(editor: Editor, text: string) {
  const { createMermaidDiagram } = await import("@tldraw/mermaid")
  await createMermaidDiagram(editor, text, {
    async onUnsupportedDiagram(svgString) {
      await editor.putExternalContent({ type: "svg-text", text: svgString })
    },
  })
}
