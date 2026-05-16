"use client"

import { useParams } from "next/navigation"
import { useWorkspaceUrlState } from "@/modules/workspace/api/use-workspace-url-state"

export default function WorkspacePage() {
  const params = useParams<{ id?: string | string[] }>()
  const workspaceId = Array.isArray(params?.id) ? params.id[0] : (params?.id ?? "")
  useWorkspaceUrlState(workspaceId)

  return (
    <div className="flex min-h-full flex-1 items-center justify-center p-6">
      <div className="max-w-md rounded-2xl border border-dashed bg-card p-6 text-center shadow-sm">
        <p className="text-xs uppercase tracking-[0.2em] text-muted-foreground">
          Workspace
        </p>
        <h1 className="mt-2 text-2xl font-semibold tracking-tight">
          {workspaceId || "Unknown workspace"}
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">
          The selected workspace is kept in the URL with nuqs so direct links and sidebar
          selection stay in sync.
        </p>
      </div>
    </div>
  )
}
