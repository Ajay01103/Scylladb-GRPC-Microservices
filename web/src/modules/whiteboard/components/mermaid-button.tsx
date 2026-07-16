"use client"

import { TldrawUiToolbarButton, type Editor } from "tldraw"

import { MermaidDialog } from "./mermaid-dialog"

/**
 * Toolbar button shown inside tldraw's DefaultToolbar. Clicking it asks
 * the parent to open the Mermaid dialog. Only the tldraw-styled button
 * stays in the toolbar slot; the actual <MermaidDialog /> is rendered
 * in MermaidDialogHost.
 *
 * This is necessary because tldraw's DefaultToolbar iterates its children
 * and only renders recognized tldraw UI primitives. A Radix <Dialog />
 * sibling there gets swallowed. The dialog is instead rendered in the
 * outer page tree (see MermaidDialogHost) where its portal target —
 * <body> — sits above tldraw's stacked overlays and z-index layers.
 *
 * Open state is owned by `WhiteboardCanvas` (the closest common parent
 * of the toolbar slot and the dialog host) and threaded in as props.
 */
export function MermaidToolbarButton({ onOpen }: { onOpen: () => void }) {
  return (
    <TldrawUiToolbarButton
      className="font-medium tracking-wide"
      onClick={onOpen}
      title="Insert Mermaid"
      type="menu"
    >
      Mermaid
    </TldrawUiToolbarButton>
  )
}

/**
 * Mount this once, outside the <Tldraw> component. Receives the open
 * state from the parent and renders the actual <MermaidDialog />, which
 * uses a Radix portal at document.body so positioning, z-index, and
 * focus traps all work correctly.
 */
export function MermaidDialogHost({
  editor,
  open,
  onOpenChange,
}: {
  editor: Editor | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  if (!editor) return null

  return <MermaidDialog editor={editor} open={open} onOpenChange={onOpenChange} />
}
