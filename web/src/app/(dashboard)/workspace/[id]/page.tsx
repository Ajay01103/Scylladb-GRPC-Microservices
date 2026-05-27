"use client"

import { useEffect } from "react"

import { useParams, useRouter } from "next/navigation"

import { WorkspaceView } from "@/modules/workspace/components/workspace-view"
import { useWorkspace } from "@/modules/workspace/api/use-workspaces"

export default function WorkspacePage() {
  const router = useRouter()
  const params = useParams<{ id?: string | string[] }>()
  const workspaceId = Array.isArray(params?.id) ? params.id[0] : (params?.id ?? "")
  const workspaceQuery = useWorkspace(workspaceId)

  useEffect(() => {
    if (!workspaceId) {
      router.replace("/workspace")
    }
  }, [router, workspaceId])

  useEffect(() => {
    if (!workspaceQuery.isLoading && !workspaceQuery.data) {
      router.replace("/workspace")
    }
  }, [router, workspaceQuery.data, workspaceQuery.isLoading])

  if (workspaceQuery.isLoading || !workspaceQuery.data) {
    return <WorkspaceView />
  }

  return <WorkspaceView workspace={workspaceQuery.data} />
}
