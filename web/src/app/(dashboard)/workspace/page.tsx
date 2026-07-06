"use client"

import { useEffect } from "react"

import { useRouter } from "next/navigation"

import { useMyWorkspaces } from "@/modules/workspace/api/use-workspaces"

export default function WorkspaceLandingPage() {
  const router = useRouter()
  const workspacesQuery = useMyWorkspaces()

  useEffect(() => {
    const firstWorkspace = workspacesQuery.data?.[0]

    if (firstWorkspace) {
      router.replace(`/workspace/${firstWorkspace.id}`)
    }
  }, [router, workspacesQuery.data])

  if (workspacesQuery.isLoading) {
    return (
      <div className="flex min-h-full flex-1 items-center justify-center text-sm text-muted-foreground">
        Loading your workspaces...
      </div>
    )
  }

  if (!workspacesQuery.data?.length) {
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

  return null
}