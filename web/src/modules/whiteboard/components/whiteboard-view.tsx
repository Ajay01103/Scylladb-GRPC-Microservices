"use client"

import { useWhiteboardBySlugSuspense } from "@/modules/whiteboard/api/use-whiteboards"

import { WhiteboardCanvas } from "@/modules/whiteboard/components/whiteboard-canvas"

type WhiteboardViewProps = {
  slug: string
}

export function WhiteboardView({ slug }: WhiteboardViewProps) {
  const { board, workspaceId } = useWhiteboardBySlugSuspense(slug)

  return <WhiteboardCanvas boardId={board.id} workspaceId={workspaceId} slug={slug} />
}
