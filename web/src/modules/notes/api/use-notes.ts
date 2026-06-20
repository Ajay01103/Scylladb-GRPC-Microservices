"use client"

import { create } from "@bufbuild/protobuf"
import { useMemo } from "react"
import {
  useMutation,
  useQueries,
  useQuery,
  useQueryClient,
  type UseQueryResult,
} from "@tanstack/react-query"
import { generateSlug } from "random-word-slugs"

import { CreateNoteRequestSchema, type Note } from "@/gen/pb/notes/notes_pb"
import { notesRpcClient } from "@/lib/rpc"
import { useIsLoadingAuth } from "@/lib/token-store"
import { useMyWorkspaces, workspaceLibraryQueryKey } from "@/modules/workspace/api/use-workspaces"

export const notesQueryKey = ["notes"] as const

type CreateNoteInput = {
  workspaceId: string
}

export function useCreateNote() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async ({ workspaceId }: CreateNoteInput): Promise<Note> => {
      const title = generateSlug(2, { format: "kebab" })

      return notesRpcClient.createNote(
        create(CreateNoteRequestSchema, {
          workspaceId,
          title,
          isPrivate: false,
        }),
      )
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: workspaceLibraryQueryKey })
    },
  })
}

export function useNote(noteId: string) {
  const isLoadingAuth = useIsLoadingAuth()

  return useQuery({
    queryKey: [...notesQueryKey, noteId],
    enabled: !isLoadingAuth && noteId.length > 0,
    staleTime: 60 * 1000,
    gcTime: 10 * 60 * 1000,
    refetchOnWindowFocus: false,
    retry: 1,
    queryFn: async (): Promise<Note> => {
      return notesRpcClient.getNote({ noteId })
    },
  })
}

type NoteMatch = {
  note: Note
  workspaceId: string
}

/**
 * Finds a note by its title slug across all workspaces the current user belongs to.
 * Mirrors useWhiteboardBySlug — no backend slug endpoint needed.
 */
export function useNoteBySlug(slug?: string) {
  const safeSlug = slug ?? ""
  const workspacesQuery = useMyWorkspaces()
  const isLoadingAuth = useIsLoadingAuth()

  const workspaceIds = useMemo(
    () => (workspacesQuery.data ?? []).map((w) => w.id),
    [workspacesQuery.data],
  )

  const noteQueries = useQueries({
    queries: workspaceIds.map((workspaceId) => ({
      // Distinct key from useWorkspaceNotes ([workspaceLibraryQueryKey, workspaceId, "notes"])
      // which stores WorkspaceLibraryItem[]. This query stores raw Note[] — mixing the two
      // shapes in the same cache entry causes "Objects are not valid as React child" when
      // the Timestamp protobuf object lands in the table's updatedAt cell.
      queryKey: [...workspaceLibraryQueryKey, workspaceId, "notes-raw"],
      enabled:
        safeSlug.length > 0 &&
        workspaceId.length > 0 &&
        !workspacesQuery.isLoading &&
        !isLoadingAuth,
      staleTime: 60 * 1000,
      gcTime: 10 * 60 * 1000,
      refetchOnWindowFocus: false,
      retry: 1,
      queryFn: async (): Promise<Note[]> => {
        const response = await notesRpcClient.listWorkspaceNotes({ workspaceId })
        return response.notes
      },
    })),
  }) as UseQueryResult<Note[]>[]

  const matchedNote = useMemo<NoteMatch | null>(() => {
    for (let i = 0; i < noteQueries.length; i++) {
      const workspaceId = workspaceIds[i]
      const items = noteQueries[i]?.data ?? []
      const match = items.find((n) => n.title === safeSlug)
      if (match) return { note: match, workspaceId }
    }
    return null
  }, [noteQueries, safeSlug, workspaceIds])

  const isLoadingNotes = noteQueries.some((q) => q.isLoading || q.isFetching)

  return {
    note: matchedNote?.note ?? null,
    workspaceId: matchedNote?.workspaceId ?? null,
    isLoading: isLoadingAuth || workspacesQuery.isLoading || isLoadingNotes,
    isError: workspacesQuery.isError || noteQueries.some((q) => q.isError),
  }
}
