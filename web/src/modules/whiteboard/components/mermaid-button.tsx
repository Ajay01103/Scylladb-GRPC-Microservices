"use client"

import { atom, useAtom, useSetAtom } from "jotai"

import { TldrawUiToolbarButton, type Editor } from "tldraw"

import { MermaidDialog } from "./mermaid-dialog"

// Module-scoped atom: lets the toolbar button (which lives inside
// tldraw's <Tldraw> UI tree) and the dialog (rendered outside <Tldraw>)
// share open-state without prop-drilling through tldraw's slot system.
//
// This is necessary because tldraw's DefaultToolbar iterates its children
// and only renders recognized tldraw UI primitives. A Radix <Dialog />
// sibling there gets swallowed. The dialog is instead rendered in the
// outer page tree (see MermaidDialogHost) where its portal target —
// <body> — sits above tldraw's stacked overlays and z-index layers.
const mermaidDialogOpenAtom = atom(false)

/**
 * Toolbar button shown inside tldraw's DefaultToolbar. Clicking it flips
 * the open-state atom. Only the tldraw-styled button stays in the toolbar
 * slot; the actual <MermaidDialog /> is rendered in MermaidDialogHost.
 */
export function MermaidToolbarButton() {
  const setOpen = useSetAtom(mermaidDialogOpenAtom)

  return (
    <TldrawUiToolbarButton
      className="font-medium tracking-wide"
      onClick={() => setOpen(true)}
      title="Insert Mermaid"
      type="menu"
    >
      Mermaid
    </TldrawUiToolbarButton>
  )
}

/**
 * Mount this once, outside the <Tldraw> component. It subscribes to the
 * open-state atom and renders the actual <MermaidDialog />, which uses
 * a Radix portal at document.body so positioning, z-index, and focus
 * traps all work correctly.
 */
export function MermaidDialogHost({ editor }: { editor: Editor | null }) {
  const [open, setOpen] = useAtom(mermaidDialogOpenAtom)

  if (!editor) return null

  return <MermaidDialog editor={editor} open={open} onOpenChange={setOpen} />
}
