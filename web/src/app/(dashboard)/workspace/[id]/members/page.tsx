"use client"

import { useParams } from "next/navigation"

import { useWorkspace, useWorkspaceMembers } from "@/modules/workspace/api/use-workspaces"
import { WorkspaceMembersView } from "@/modules/workspace/components/workspace-members-view"

export default function WorkspaceMembersPage() {
  const params = useParams<{ id?: string | string[] }>()
  const workspaceId = Array.isArray(params?.id) ? params.id[0] : (params?.id ?? "")
  const workspaceQuery = useWorkspace(workspaceId)
  const membersQuery = useWorkspaceMembers(workspaceId, Boolean(workspaceQuery.data))

  if (workspaceQuery.isLoading || !workspaceQuery.data) {
    return <WorkspaceMembersView isMembersLoading={membersQuery.isLoading} />
  }

  return (
    <WorkspaceMembersView
      isMembersLoading={membersQuery.isLoading}
      members={membersQuery.data ?? []}
      workspace={workspaceQuery.data}
    />
  )
}
