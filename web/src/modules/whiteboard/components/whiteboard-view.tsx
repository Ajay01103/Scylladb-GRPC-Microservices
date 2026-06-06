"use client"

import { Loader2 } from "lucide-react"

import { useWhiteboardBySlug } from "@/modules/whiteboard/api/use-whiteboards"

import { WhiteboardCanvas } from "@/modules/whiteboard/components/whiteboard-canvas"

type WhiteboardViewProps = {
  slug?: string
}

export function WhiteboardView({ slug }: WhiteboardViewProps) {
  const whiteboardQuery = useWhiteboardBySlug(slug)
  const safeSlug = slug ?? ""

  if (whiteboardQuery.isLoading) {
    return (
      <div className="flex h-svh w-full items-center justify-center bg-background text-sm text-muted-foreground">
        <div className="flex items-center gap-2">
          <Loader2 className="size-4 animate-spin" />
          Loading whiteboard...
        </div>
      </div>
    )
  }

  if (!whiteboardQuery.board || !whiteboardQuery.workspaceId) {
    return (
      <div className="flex h-svh w-full items-center justify-center bg-background text-sm text-muted-foreground">
        Whiteboard not found.
      </div>
    )
  }

  return (
    <WhiteboardCanvas
      boardId={whiteboardQuery.board.id}
      workspaceId={whiteboardQuery.workspaceId}
      slug={safeSlug}
    />
  )
}
