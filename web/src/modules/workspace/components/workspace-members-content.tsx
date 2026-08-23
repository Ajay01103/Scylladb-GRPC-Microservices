"use client"

import { useSuspenseQuery } from "@tanstack/react-query"

import {
  workspaceMembersQueryOptions,
  workspaceQueryOptions,
} from "@/modules/workspace/api/use-workspaces"
import { WorkspaceMembersView } from "@/modules/workspace/components/workspace-members-view"

export function WorkspaceMembersContent({ workspaceId }: { workspaceId: string }) {
  const { data: workspace } = useSuspenseQuery(workspaceQueryOptions(workspaceId))
  const { data: members } = useSuspenseQuery(workspaceMembersQueryOptions(workspaceId))

  return <WorkspaceMembersView members={members ?? []} workspace={workspace} />
}
