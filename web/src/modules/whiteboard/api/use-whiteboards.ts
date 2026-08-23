"use client"

import { create } from "@bufbuild/protobuf"
import { useMemo } from "react"
import {
  useMutation,
  useQueries,
  useQueryClient,
  useSuspenseQuery,
  type UseQueryResult,
} from "@tanstack/react-query"
import { generateSlug } from "random-word-slugs"

import { CreateBoardRequestSchema, type Board } from "@/gen/pb/whiteboard/whiteboard_pb"
import { whiteboardRpcClient } from "@/lib/rpc"
import { useMyWorkspaces, workspaceLibraryQueryKey } from "@/modules/workspace/api/use-workspaces"
import { makeWhiteboardBySlugQueryOptions } from "@/modules/whiteboard/api/whiteboard-queries"

export function createWhiteboardSlug() {
  return generateSlug(2, { format: "kebab" })
}

export function buildWhiteboardPath(slug: string) {
  return `/whiteboard/${encodeURIComponent(slug)}`
}

export type CreateWhiteboardInput = {
  workspaceId: string
}

export function useCreateWhiteboard() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async ({ workspaceId }: CreateWhiteboardInput): Promise<Board> => {
      const slug = createWhiteboardSlug()

      return whiteboardRpcClient.createBoard(
        create(CreateBoardRequestSchema, {
          workspaceId,
          title: slug,
          isPrivate: false,
        }),
      )
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: workspaceLibraryQueryKey })
    },
  })
}

type WhiteboardMatch = {
  board: Board
  workspaceId: string
}

export function useWhiteboardBySlug(slug?: string) {
  const safeSlug = slug ?? ""
  const workspacesQuery = useMyWorkspaces()
  const workspaceIds = useMemo(
    () => (workspacesQuery.data ?? []).map((workspace) => workspace.id),
    [workspacesQuery.data],
  )

  const boardQueries = useQueries({
    queries: workspaceIds.map((workspaceId) => ({
      queryKey: [...workspaceLibraryQueryKey, workspaceId, "boards"],
      enabled: safeSlug.length > 0 && workspaceId.length > 0 && !workspacesQuery.isLoading,
      staleTime: 60 * 1000,
      gcTime: 10 * 60 * 1000,
      refetchOnWindowFocus: false,
      retry: 1,
      queryFn: async (): Promise<Board[]> => {
        const response = await whiteboardRpcClient.listWorkspaceBoards({ workspaceId })

        return response.boards
      },
    })),
  }) as UseQueryResult<Board[]>[]

  const matchedBoard = useMemo<WhiteboardMatch | null>(() => {
    for (let index = 0; index < boardQueries.length; index += 1) {
      const workspaceId = workspaceIds[index]
      const items = boardQueries[index]?.data ?? []
      const match = items.find((item) => item.title === safeSlug)

      if (match) {
        return {
          board: match,
          workspaceId,
        }
      }
    }

    return null
  }, [boardQueries, safeSlug, workspaceIds])

  const isLoadingBoards = boardQueries.length === 0 || boardQueries.some((query) => query.isLoading)

  return {
    board: matchedBoard?.board ?? null,
    workspaceId: matchedBoard?.workspaceId ?? null,
    isLoading: workspacesQuery.isLoading || isLoadingBoards,
    isError: workspacesQuery.isError || boardQueries.some((query) => query.isError),
  }
}

/**
 * Suspense variant for whiteboard by slug.
 * Only use inside a component wrapped in a <Suspense> boundary
 * (e.g. fed by HydrationBoundary from a server prefetch).
 */
export function useWhiteboardBySlugSuspense(slug: string) {
  const { data } = useSuspenseQuery(makeWhiteboardBySlugQueryOptions({ whiteboardRpcClient }, slug))
  return {
    board: data.board,
    workspaceId: data.workspaceId,
  }
}
