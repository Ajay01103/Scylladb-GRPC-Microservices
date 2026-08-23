// src/modules/whiteboard/api/whiteboard-queries.ts
//
// No "use client", no hardcoded RPC client. Every factory takes the
// clients it needs as an argument, so the exact same queryKey + queryFn
// shape can be used both from client hooks (bound to the browser
// singletons in rpc.ts) and from server prefetchQuery calls (bound to the
// per-request clients in rpc-server.ts). If these ever diverged, the
// dehydrated server cache and the client hook would disagree and you'd
// get a silent refetch or a hydration mismatch.

import { queryOptions } from "@tanstack/react-query"
import type { Client } from "@connectrpc/connect"

import type { Board } from "@/gen/pb/whiteboard/whiteboard_pb"
import { WhiteboardService } from "@/gen/pb/whiteboard/whiteboard_pb"

export type WhiteboardQueryClients = {
  whiteboardRpcClient: Client<typeof WhiteboardService>
}

export type WhiteboardBySlugResult = {
  board: Board
  workspaceId: string
}

export function makeWhiteboardBySlugQueryOptions(
  { whiteboardRpcClient }: WhiteboardQueryClients,
  slug: string,
) {
  return queryOptions({
    queryKey: ["whiteboard", "bySlug", slug],
    enabled: slug.length > 0,
    staleTime: 60 * 1000,
    gcTime: 10 * 60 * 1000,
    refetchOnWindowFocus: false,
    retry: 1,
    queryFn: async (): Promise<WhiteboardBySlugResult> => {
      const response = await whiteboardRpcClient.getBoardBySlug({ slug })
      if (!response.board) {
        throw new Error("whiteboard not found")
      }
      return { board: response.board, workspaceId: response.workspaceId }
    },
  })
}
