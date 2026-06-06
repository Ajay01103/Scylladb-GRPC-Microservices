"use client"

import { useState, useTransition } from "react"

import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Textarea } from "@/components/ui/textarea"
import { Button } from "@/components/ui/button"
import type { Editor } from "tldraw"

import { insertMermaidDiagram } from "./mermaid-utils"

const MERMAID_SAMPLE = `flowchart LR
    A[Start] --> B{Is it Friday?}
    B -- Yes --> C[🎉 Party]
    B -- No --> D[Keep working]`

type MermaidDialogProps = {
  editor: Editor
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function MermaidDialog({ editor, open, onOpenChange }: MermaidDialogProps) {
  const [value, setValue] = useState("")
  const [error, setError] = useState<string | null>(null)
  const [isPending, startTransition] = useTransition()

  const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const text = value.trim()
    if (!text) {
      setError("Please paste some Mermaid code first.")
      return
    }

    setError(null)
    startTransition(async () => {
      try {
        await insertMermaidDiagram(editor, text)
        setValue("")
        onOpenChange(false)
      } catch (err) {
        setError(
          err instanceof Error
            ? `Could not render diagram: ${err.message}`
            : "Could not render the Mermaid diagram. Please check your syntax.",
        )
      }
    })
  }

  const handleOpenChange = (next: boolean) => {
    if (!next) {
      setError(null)
    }
    onOpenChange(next)
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Insert Mermaid diagram</DialogTitle>
          <DialogDescription>
            Paste Mermaid syntax below. Supported types: flowchart, sequence, state, class, ER,
            gantt, pie, git, and mindmap.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="flex flex-col gap-3">
          <Textarea
            autoFocus
            value={value}
            onChange={(event) => {
              setValue(event.target.value)
              if (error) setError(null)
            }}
            placeholder={MERMAID_SAMPLE}
            rows={12}
            spellCheck={false}
            className="max-h-80 min-h-48 resize-y font-mono text-xs leading-relaxed"
            aria-label="Mermaid diagram source"
            aria-invalid={Boolean(error)}
            data-testid="mermaid-source"
          />

          {error ? (
            <p role="alert" className="text-xs text-semantic-error">
              {error}
            </p>
          ) : null}

          <DialogFooter className="-mx-4 -mb-4">
            <DialogClose asChild>
              <Button type="button" variant="outline" disabled={isPending}>
                Cancel
              </Button>
            </DialogClose>
            <Button type="submit" disabled={isPending || !value.trim()}>
              {isPending ? "Inserting…" : "Insert diagram"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
