// src/modules/workspace/api/workspace-queries.ts
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

import type { Note } from "@/gen/pb/notes/notes_pb"
import { NotesService } from "@/gen/pb/notes/notes_pb"
import type { Board } from "@/gen/pb/whiteboard/whiteboard_pb"
import { WhiteboardService } from "@/gen/pb/whiteboard/whiteboard_pb"
import {
  WorkspaceRole,
  WorkspaceService,
  type Workspace,
  type WorkspaceMember,
} from "@/gen/pb/workspace/workspace_pb"

export type PlainWorkspace = {
  id: string
  name: string
  slug: string
  description: string
  iconUrl: string
  ownerId: string
  isPublic: boolean
  createdAt: string
  updatedAt: string
  myRole: WorkspaceRole
}

function toPlainWorkspace(w: Workspace): PlainWorkspace {
  return {
    id: w.id,
    name: w.name,
    slug: w.slug,
    description: w.description,
    iconUrl: w.iconUrl,
    ownerId: w.ownerId,
    isPublic: w.isPublic,
    createdAt: formatWorkspaceLibraryTimestamp(w.createdAt),
    updatedAt: formatWorkspaceLibraryTimestamp(w.updatedAt),
    myRole: w.myRole,
  }
}

export type WorkspaceQueryClients = {
  workspaceRpcClient: Client<typeof WorkspaceService>
  notesRpcClient: Client<typeof NotesService>
  whiteboardRpcClient: Client<typeof WhiteboardService>
}

export const workspaceQueryKey = ["myWorkspaces"] as const
export const workspaceLibraryQueryKey = ["workspaceLibrary"] as const

export function formatWorkspaceLibraryTimestamp(timestamp?: {
  seconds?: bigint | number | string
  nanos?: number
  toDate?: () => Date
}) {
  if (!timestamp) return "—"
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
  if (Number.isNaN(date.getTime())) return "—"
  return date.toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" })
}

export type WorkspaceLibraryItem = {
  id: string
  title: string
  createdBy: string
  visibility: string
  updatedAt: string
}

export function makeWorkspacesQueryOptions({ workspaceRpcClient }: WorkspaceQueryClients) {
  return queryOptions({
    queryKey: workspaceQueryKey,
    queryFn: async (): Promise<PlainWorkspace[]> => {
      const response = await workspaceRpcClient.listMyWorkspaces({ pageSize: 100, pageToken: "" })
      return response.workspaces.map(toPlainWorkspace)
    },
  })
}

export function makeWorkspaceQueryOptions(
  { workspaceRpcClient }: WorkspaceQueryClients,
  workspaceId: string,
) {
  return queryOptions({
    queryKey: [...workspaceQueryKey, workspaceId],
    queryFn: async (): Promise<PlainWorkspace> =>
      toPlainWorkspace(await workspaceRpcClient.getWorkspace({ workspaceId })),
  })
}

export function makeWorkspaceMembersQueryOptions(
  { workspaceRpcClient }: WorkspaceQueryClients,
  workspaceId: string,
) {
  return queryOptions({
    queryKey: [...workspaceQueryKey, workspaceId, "members"],
    queryFn: async (): Promise<WorkspaceMember[]> => {
      const response = await workspaceRpcClient.listMembers({ workspaceId })
      return response.members
    },
  })
}

export function makeWorkspaceNotesQueryOptions(
  { notesRpcClient }: WorkspaceQueryClients,
  workspaceId: string,
) {
  return queryOptions({
    queryKey: [...workspaceLibraryQueryKey, workspaceId, "notes"],
    queryFn: async (): Promise<WorkspaceLibraryItem[]> => {
      const response = await notesRpcClient.listWorkspaceNotes({ workspaceId })
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

export function makeWorkspaceBoardsQueryOptions(
  { whiteboardRpcClient }: WorkspaceQueryClients,
  workspaceId: string,
) {
  return queryOptions({
    queryKey: [...workspaceLibraryQueryKey, workspaceId, "boards"],
    queryFn: async (): Promise<WorkspaceLibraryItem[]> => {
      const response = await whiteboardRpcClient.listWorkspaceBoards({ workspaceId })
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
