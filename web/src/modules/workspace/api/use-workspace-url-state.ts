"use client"

import { useEffect } from "react"
import { parseAsString, useQueryState } from "nuqs"

export function useWorkspaceUrlState(routeWorkspaceId: string) {
  const [workspaceQuery, setWorkspaceQuery] = useQueryState(
    "workspace",
    parseAsString.withDefault(""),
  )

  useEffect(() => {
    if (routeWorkspaceId && routeWorkspaceId !== workspaceQuery) {
      void setWorkspaceQuery(routeWorkspaceId)
    }
  }, [routeWorkspaceId, setWorkspaceQuery, workspaceQuery])

  return {
    selectedWorkspaceId: routeWorkspaceId || workspaceQuery,
    setWorkspaceQuery,
    workspaceQuery,
  }
}
