import { Suspense } from "react"
import { dehydrate, HydrationBoundary } from "@tanstack/react-query"
import { redirect } from "next/navigation"

import { getQueryClient } from "@/lib/get-query-client"
import { getServerRpcClients } from "@/lib/rpc-server"
import {
  prefetchWorkspace,
  prefetchWorkspaceMembers,
} from "@/modules/workspace/api/workspace-server-queries"

import { MembersSkeleton } from "@/modules/workspace/components/members-skeleton"
import { WorkspaceMembersContent } from "@/modules/workspace/components/workspace-members-content"

export default async function WorkspaceMembersPage({
  params,
}: {
  params: Promise<{ id: string }>
}) {
  const { id: workspaceId } = await params
  const queryClient = getQueryClient()

  const { token } = await getServerRpcClients()
  if (!token) redirect("/login")

  await Promise.all([
    prefetchWorkspace(queryClient, workspaceId),
    prefetchWorkspaceMembers(queryClient, workspaceId),
  ])

  return (
    <HydrationBoundary state={dehydrate(queryClient)}>
      <Suspense fallback={<MembersSkeleton />}>
        <WorkspaceMembersContent workspaceId={workspaceId} />
      </Suspense>
    </HydrationBoundary>
  )
}
