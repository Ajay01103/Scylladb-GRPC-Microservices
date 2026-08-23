import "server-only"

import type { QueryClient } from "@tanstack/react-query"

import { getServerRpcClients } from "@/lib/rpc-server"
import { makeNoteBySlugQueryOptions } from "./notes-queries"

export async function prefetchNoteBySlug(queryClient: QueryClient, slug: string) {
  const { notesClient } = await getServerRpcClients()
  return queryClient.prefetchQuery(
    makeNoteBySlugQueryOptions({ notesRpcClient: notesClient }, slug),
  )
}
