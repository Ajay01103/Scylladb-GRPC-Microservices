import { redirect } from "next/navigation"

import { getQueryClient } from "@/lib/get-query-client"
import {
  prefetchWorkspaces,
} from "@/modules/workspace/api/workspace-server-queries"

export const dynamic = "force-dynamic"

export default async function WorkspaceLandingPage() {
  const queryClient = getQueryClient()
  await prefetchWorkspaces(queryClient)

  const workspaces = queryClient.getQueryData(["myWorkspaces"]) as
    | Array<{ id: string }>
    | undefined

  if (!workspaces || workspaces.length === 0) {
    return (
      <div className="flex min-h-full flex-1 items-center justify-center p-6">
        <div className="max-w-md rounded-2xl border border-dashed bg-card p-6 text-center shadow-sm">
          <p className="text-xs uppercase tracking-[0.2em] text-muted-foreground">Workspace</p>
          <h1 className="mt-2 text-2xl font-semibold tracking-tight">
            Create your first workspace
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Use the workspace picker in the sidebar to create a workspace and start here.
          </p>
        </div>
      </div>
    )
  }

  redirect(`/workspace/${workspaces[0].id}`)
}
