"use client"

import { create } from "@bufbuild/protobuf"
import { useMutation, useQuery, useQueryClient, useSuspenseQuery } from "@tanstack/react-query"

import { notesRpcClient, workspaceRpcClient, whiteboardRpcClient } from "@/lib/rpc"
import { useAuth } from "@/lib/auth-context"
import {
  workspaceQueryKey,
  workspaceLibraryQueryKey,
  makeWorkspacesQueryOptions,
  makeWorkspaceQueryOptions,
  makeWorkspaceMembersQueryOptions,
  makeWorkspaceNotesQueryOptions,
  makeWorkspaceBoardsQueryOptions,
  formatWorkspaceLibraryTimestamp,
  type WorkspaceLibraryItem,
  type PlainWorkspace,
} from "@/modules/workspace/api/workspace-queries"
import {
  CreateWorkspaceRequestSchema,
  WorkspaceRole,
  type Workspace,
} from "@/gen/pb/workspace/workspace_pb"

// ─── Browser-bound clients ────────────────────────────────────────────────────
// Passed into the shared query-option factories so the same queryKey + queryFn
// shape is used by both client hooks and server prefetchQuery calls.
// Server-side prefetch uses rpc-server.ts clients instead.

const browserClients = { workspaceRpcClient, notesRpcClient, whiteboardRpcClient }

// ─── Query options (re-exported for use in prefetchQuery / HydrationBoundary) ─

export const workspacesQueryOptions = makeWorkspacesQueryOptions(browserClients)

export const workspaceQueryOptions = (workspaceId: string) =>
  makeWorkspaceQueryOptions(browserClients, workspaceId)

export const workspaceMembersQueryOptions = (workspaceId: string) =>
  makeWorkspaceMembersQueryOptions(browserClients, workspaceId)

export const workspaceNotesQueryOptions = (workspaceId: string) =>
  makeWorkspaceNotesQueryOptions(browserClients, workspaceId)

export const workspaceBoardsQueryOptions = (workspaceId: string) =>
  makeWorkspaceBoardsQueryOptions(browserClients, workspaceId)

// ─── Re-exports from workspace-queries (shared between client + server) ───────

export { workspaceQueryKey, workspaceLibraryQueryKey, formatWorkspaceLibraryTimestamp }
export type { WorkspaceLibraryItem, PlainWorkspace }

// ─── Hooks ────────────────────────────────────────────────────────────────────

export function useMyWorkspaces() {
  const { isLoadingAuth } = useAuth()

  return useQuery({
    ...workspacesQueryOptions,
    enabled: !isLoadingAuth,
    refetchOnWindowFocus: false,
  })
}

export function useWorkspace(workspaceId: string) {
  const { isLoadingAuth } = useAuth()

  return useQuery({
    ...workspaceQueryOptions(workspaceId),
    enabled: !isLoadingAuth && workspaceId.length > 0,
    refetchOnWindowFocus: false,
  })
}

/**
 * Suspense variant for workspace detail.
 * Only use inside a component wrapped in a <Suspense> boundary
 * (e.g. fed by HydrationBoundary from a server prefetch).
 */
export function useSuspenseWorkspace(workspaceId: string) {
  return useSuspenseQuery(workspaceQueryOptions(workspaceId))
}

export function useWorkspaceMembers(workspaceId: string, enabled = true) {
  const { isLoadingAuth } = useAuth()

  return useQuery({
    ...workspaceMembersQueryOptions(workspaceId),
    enabled: enabled && !isLoadingAuth && workspaceId.length > 0,
    refetchOnWindowFocus: false,
  })
}

export function useWorkspaceNotes(workspaceId: string, enabled = true) {
  const { isLoadingAuth } = useAuth()

  return useQuery({
    ...workspaceNotesQueryOptions(workspaceId),
    enabled: enabled && !isLoadingAuth && workspaceId.length > 0,
    refetchOnWindowFocus: false,
  })
}

export function useWorkspaceBoards(workspaceId: string, enabled = true) {
  const { isLoadingAuth } = useAuth()

  return useQuery({
    ...workspaceBoardsQueryOptions(workspaceId),
    enabled: enabled && !isLoadingAuth && workspaceId.length > 0,
    refetchOnWindowFocus: false,
  })
}

// ─── Mutations ────────────────────────────────────────────────────────────────

export function buildWorkspaceSlug(name: string) {
  const normalized = name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")

  return normalized || "workspace"
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
    mutationFn: async (input: CreateWorkspaceInput) =>
      workspaceRpcClient.createWorkspace(create(CreateWorkspaceRequestSchema, input)),
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
      const response = await workspaceRpcClient.acceptInvitation({ token: inviteCode })
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
      await workspaceRpcClient.rejectInvitation({ token: inviteCode })
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
    mutationFn: async ({ workspaceId, userId }: LeaveWorkspaceInput) =>
      workspaceRpcClient.removeMember({ workspaceId, userId }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: workspaceQueryKey })
    },
  })
}
