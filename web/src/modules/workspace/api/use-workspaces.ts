"use client"

import { create } from "@bufbuild/protobuf"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { notesRpcClient } from "@/lib/rpc"
import { workspaceRpcClient } from "@/lib/rpc"
import { whiteboardRpcClient } from "@/lib/rpc"
import { useIsLoadingAuth } from "@/lib/token-store"
import type { Board } from "@/gen/pb/whiteboard/whiteboard_pb"
import type { Note } from "@/gen/pb/notes/notes_pb"
import {
  CreateWorkspaceRequestSchema,
  WorkspaceRole,
  type WorkspaceMember,
  type Workspace,
} from "@/gen/pb/workspace/workspace_pb"

export const workspaceQueryKey = ["myWorkspaces"] as const
export const workspaceLibraryQueryKey = ["workspaceLibrary"] as const

export function buildWorkspaceSlug(name: string) {
  const normalized = name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")

  return normalized || "workspace"
}

export function useMyWorkspaces() {
  const isLoadingAuth = useIsLoadingAuth()

  return useQuery({
    queryKey: workspaceQueryKey,
    enabled: !isLoadingAuth,
    staleTime: 5 * 60 * 1000,
    gcTime: 30 * 60 * 1000,
    refetchOnWindowFocus: false,
    retry: 1,
    queryFn: async (): Promise<Workspace[]> => {
      const response = await workspaceRpcClient.listMyWorkspaces({
        pageSize: 100,
        pageToken: "",
      })

      return response.workspaces
    },
  })
}

export function useWorkspace(workspaceId: string) {
  const isLoadingAuth = useIsLoadingAuth()

  return useQuery({
    queryKey: [...workspaceQueryKey, workspaceId],
    enabled: !isLoadingAuth && workspaceId.length > 0,
    staleTime: 5 * 60 * 1000,
    gcTime: 30 * 60 * 1000,
    refetchOnWindowFocus: false,
    retry: 1,
    queryFn: async (): Promise<Workspace> => {
      return workspaceRpcClient.getWorkspace({
        workspaceId,
      })
    },
  })
}

export function useWorkspaceMembers(workspaceId: string, enabled = true) {
  const isLoadingAuth = useIsLoadingAuth()

  return useQuery({
    queryKey: [...workspaceQueryKey, workspaceId, "members"],
    enabled: enabled && !isLoadingAuth && workspaceId.length > 0,
    staleTime: 60 * 1000,
    gcTime: 10 * 60 * 1000,
    refetchOnWindowFocus: false,
    retry: 1,
    queryFn: async (): Promise<WorkspaceMember[]> => {
      const response = await workspaceRpcClient.listMembers({
        workspaceId,
      })

      return response.members
    },
  })
}

function formatWorkspaceLibraryTimestamp(timestamp?: {
  seconds?: bigint | number | string
  nanos?: number
  toDate?: () => Date
}) {
  if (!timestamp) {
    return "—"
  }

  if (typeof timestamp.toDate === "function") {
    return timestamp.toDate().toLocaleDateString(undefined, {
      month: "short",
      day: "numeric",
      year: "numeric",
    })
  }

  const seconds = Number(timestamp.seconds ?? 0)
  const nanos = Number(timestamp.nanos ?? 0)
  const date = new Date(seconds * 1000 + nanos / 1_000_000)

  if (Number.isNaN(date.getTime())) {
    return "—"
  }

  return date.toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
  })
}

export type WorkspaceLibraryItem = {
  id: string
  title: string
  createdBy: string
  visibility: string
  updatedAt: string
}

export function useWorkspaceNotes(workspaceId: string, enabled = true) {
  const isLoadingAuth = useIsLoadingAuth()

  return useQuery({
    queryKey: [...workspaceLibraryQueryKey, workspaceId, "notes"],
    enabled: enabled && !isLoadingAuth && workspaceId.length > 0,
    staleTime: 60 * 1000,
    gcTime: 10 * 60 * 1000,
    refetchOnWindowFocus: false,
    retry: 1,
    queryFn: async (): Promise<WorkspaceLibraryItem[]> => {
      const response = await notesRpcClient.listWorkspaceNotes({
        workspaceId,
      })

      return response.notes.map((note: Note) => ({
        id: note.id,
        title: note.title || "Untitled note",
        createdBy: note.createdBy || "Unknown",
        visibility: note.isPrivate ? "Private" : "Shared",
        updatedAt: formatWorkspaceLibraryTimestamp(note.updatedAt),
      }))
    },
  })
}

export function useWorkspaceBoards(workspaceId: string, enabled = true) {
  const isLoadingAuth = useIsLoadingAuth()

  return useQuery({
    queryKey: [...workspaceLibraryQueryKey, workspaceId, "boards"],
    enabled: enabled && !isLoadingAuth && workspaceId.length > 0,
    staleTime: 60 * 1000,
    gcTime: 10 * 60 * 1000,
    refetchOnWindowFocus: false,
    retry: 1,
    queryFn: async (): Promise<WorkspaceLibraryItem[]> => {
      const response = await whiteboardRpcClient.listWorkspaceBoards({
        workspaceId,
      })

      return response.boards.map((board: Board) => ({
        id: board.id,
        title: board.title || "Untitled board",
        createdBy: board.createdBy || "Unknown",
        visibility: board.isPrivate ? "Private" : "Shared",
        updatedAt: formatWorkspaceLibraryTimestamp(board.updatedAt),
      }))
    },
  })
}

type CreateWorkspaceInput = {
  name: string
  slug: string
  description: string
  iconUrl: string
  isPublic: boolean
}

export function useCreateWorkspace() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (input: CreateWorkspaceInput) => {
      return workspaceRpcClient.createWorkspace(create(CreateWorkspaceRequestSchema, input))
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: workspaceQueryKey })
    },
  })
}

type GenerateWorkspaceInviteCodeInput = {
  workspaceId: string
  role?: WorkspaceRole
}

export function useGenerateWorkspaceInviteCode() {
  return useMutation({
    mutationFn: async ({
      workspaceId,
      role = WorkspaceRole.MEMBER,
    }: GenerateWorkspaceInviteCodeInput): Promise<string> => {
      const response = await workspaceRpcClient.inviteMember({
        workspaceId,
        invitedEmail: "",
        role,
      })

      return response.token
    },
  })
}

export function useAcceptWorkspaceInvitation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (inviteCode: string): Promise<Workspace> => {
      const response = await workspaceRpcClient.acceptInvitation({
        token: inviteCode,
      })

      if (!response.workspace) {
        throw new Error("invite response missing workspace")
      }

      return response.workspace
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: workspaceQueryKey })
    },
  })
}

export function useRejectWorkspaceInvitation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (inviteCode: string): Promise<void> => {
      await workspaceRpcClient.rejectInvitation({
        token: inviteCode,
      })
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: workspaceQueryKey })
    },
  })
}

type LeaveWorkspaceInput = {
  workspaceId: string
  userId: string
}

export function useLeaveWorkspace() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async ({ workspaceId, userId }: LeaveWorkspaceInput) => {
      return workspaceRpcClient.removeMember({
        workspaceId,
        userId,
      })
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: workspaceQueryKey })
    },
  })
}
