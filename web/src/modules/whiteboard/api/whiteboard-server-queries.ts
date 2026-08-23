import "server-only"

import type { QueryClient } from "@tanstack/react-query"

import { getServerRpcClients } from "@/lib/rpc-server"
import { makeWhiteboardBySlugQueryOptions } from "./whiteboard-queries"

export async function prefetchWhiteboardBySlug(queryClient: QueryClient, slug: string) {
  const { whiteboardClient } = await getServerRpcClients()
  return queryClient.prefetchQuery(
    makeWhiteboardBySlugQueryOptions({ whiteboardRpcClient: whiteboardClient }, slug),
  )
}
