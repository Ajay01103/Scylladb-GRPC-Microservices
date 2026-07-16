import { Suspense } from "react"
import { dehydrate, HydrationBoundary } from "@tanstack/react-query"
import { redirect } from "next/navigation"

import { getQueryClient } from "@/lib/get-query-client"
import { getServerRpcClients } from "@/lib/rpc-server"
import {
  prefetchWorkspace,
  prefetchWorkspaceBoards,
} from "@/modules/workspace/api/workspace-server-queries"
import { WorkspacePageContent } from "@/modules/workspace/components/workspace-page-content"
import { WorkspaceViewSkeleton } from "@/modules/workspace/components/workspace-skeleton"

export default async function WorkspacePage({
  params,
}: {
  params: Promise<{ id: string }>
}) {
  const { id: workspaceId } = await params
  const queryClient = getQueryClient()

  const { token } = await getServerRpcClients()
  if (!token) redirect("/login")

  // Parallel prefetch: workspace detail + default tab (whiteboards).
  // "notes" tab is intentionally NOT prefetched — it's not visible on first paint.
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

// "use client"

// import { useQuery } from "@tanstack/react-query"
// import { useParams } from "next/navigation"

// import { useAuth } from "@/lib/auth-context"
// import { WorkspaceView } from "@/modules/workspace/components/workspace-view"
// import { workspaceQueryOptions } from "@/modules/workspace/api/use-workspaces"

// function WorkspacePageContent({ workspaceId }: { workspaceId: string }) {
//   const { data: workspace, isLoading } = useQuery(workspaceQueryOptions(workspaceId))

//   return <WorkspaceView workspace={workspace} isLoading={isLoading} />
// }

// export default function WorkspacePage() {
//   const { isLoadingAuth } = useAuth()
//   const params = useParams<{ id?: string | string[] }>()
//   const workspaceId = Array.isArray(params?.id) ? params.id[0] : (params?.id ?? "")

//   if (isLoadingAuth || !workspaceId) {
//     return null
//   }

//   return <WorkspacePageContent workspaceId={workspaceId} />
// }
