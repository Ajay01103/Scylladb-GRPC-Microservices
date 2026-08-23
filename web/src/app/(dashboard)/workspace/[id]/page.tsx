import { Suspense } from "react"
import { dehydrate, HydrationBoundary } from "@tanstack/react-query"

import { getQueryClient } from "@/lib/get-query-client"
import {
  prefetchWorkspace,
  prefetchWorkspaceBoards,
} from "@/modules/workspace/api/workspace-server-queries"
import { WorkspacePageContent } from "@/modules/workspace/components/workspace-page-content"
import { WorkspaceViewSkeleton } from "@/modules/workspace/components/workspace-skeleton"

export default async function WorkspacePage({ params }: { params: Promise<{ id: string }> }) {
  const { id: workspaceId } = await params
  const queryClient = getQueryClient()

  await Promise.all([
    prefetchWorkspace(queryClient, workspaceId),
    prefetchWorkspaceBoards(queryClient, workspaceId),
  ])

  return (
    <HydrationBoundary state={dehydrate(queryClient)}>
      <Suspense fallback={<WorkspaceViewSkeleton />}>
        <WorkspacePageContent workspaceId={workspaceId} />
      </Suspense>
    </HydrationBoundary>
  )
}
