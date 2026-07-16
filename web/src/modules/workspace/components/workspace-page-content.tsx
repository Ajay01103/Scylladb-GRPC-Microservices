// modules/workspace/components/workspace-page-content.tsx
"use client"

import { useSuspenseQuery } from "@tanstack/react-query"

import { workspaceQueryOptions } from "@/modules/workspace/api/use-workspaces"
import { WorkspaceView } from "./workspace-view"

export function WorkspacePageContent({ workspaceId }: { workspaceId: string }) {
    // Cache hit from HydrationBoundary — no request fired on mount.
    const { data: workspace } = useSuspenseQuery(workspaceQueryOptions(workspaceId))

    return <WorkspaceView workspace={ workspace } />
}