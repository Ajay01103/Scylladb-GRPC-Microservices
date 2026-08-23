"use client"

import { create } from "@bufbuild/protobuf"
import { useMutation, useQuery, useQueryClient, useSuspenseQuery } from "@tanstack/react-query"
import { generateSlug } from "random-word-slugs"

import { CreateNoteRequestSchema, type Note } from "@/gen/pb/notes/notes_pb"
import { notesRpcClient } from "@/lib/rpc"
import { useAuth } from "@/lib/auth-context"
import { workspaceLibraryQueryKey } from "@/modules/workspace/api/use-workspaces"
import { makeNoteBySlugQueryOptions, notesQueryKey } from "@/modules/notes/api/notes-queries"

export { notesQueryKey }

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
  const { isLoadingAuth } = useAuth()

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

export function useNoteBySlug(slug?: string) {
  const safeSlug = slug ?? ""
  const { isLoadingAuth } = useAuth()

  const { data, isLoading, isError } = useQuery({
    ...makeNoteBySlugQueryOptions({ notesRpcClient }, safeSlug),
    enabled: !isLoadingAuth && safeSlug.length > 0,
  })

  return {
    note: data?.note ?? null,
    workspaceId: data?.workspaceId ?? null,
    isLoading,
    isError,
  }
}

export function useSuspenseNoteBySlug(slug: string) {
  const { data } = useSuspenseQuery(makeNoteBySlugQueryOptions({ notesRpcClient }, slug))
  return {
    note: data.note,
    workspaceId: data.workspaceId,
  }
}
