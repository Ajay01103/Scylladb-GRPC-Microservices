// modules/workspace/api/workspace-queries.server.ts
import "server-only"

import type { QueryClient } from "@tanstack/react-query"

import { getServerRpcClients } from "@/lib/rpc-server"
import {
    makeWorkspacesQueryOptions,
    makeWorkspaceQueryOptions,
    makeWorkspaceMembersQueryOptions,
    makeWorkspaceNotesQueryOptions,
    makeWorkspaceBoardsQueryOptions,
    type WorkspaceQueryClients,
} from "./workspace-queries"

async function getWorkspaceQueryClients(): Promise<WorkspaceQueryClients> {
    const { workspaceClient, notesClient, whiteboardClient } = await getServerRpcClients()
    return {
        workspaceRpcClient: workspaceClient,
        notesRpcClient: notesClient,
        whiteboardRpcClient: whiteboardClient,
    }
}

// Each function: resolve server clients → prefetch with the SAME
// queryKey the client hooks use. queryClient.prefetchQuery is a
// no-op if data isn't stale, and swallows queryFn errors into
// query state instead of throwing — safe to await in a page.

export async function prefetchWorkspaces(queryClient: QueryClient) {
    const clients = await getWorkspaceQueryClients()
    return queryClient.prefetchQuery(makeWorkspacesQueryOptions(clients))
}

export async function prefetchWorkspace(queryClient: QueryClient, workspaceId: string) {
    const clients = await getWorkspaceQueryClients()
    return queryClient.prefetchQuery(makeWorkspaceQueryOptions(clients, workspaceId))
}

export async function prefetchWorkspaceMembers(queryClient: QueryClient, workspaceId: string) {
    const clients = await getWorkspaceQueryClients()
    return queryClient.prefetchQuery(makeWorkspaceMembersQueryOptions(clients, workspaceId))
}

export async function prefetchWorkspaceNotes(queryClient: QueryClient, workspaceId: string) {
    const clients = await getWorkspaceQueryClients()
    return queryClient.prefetchQuery(makeWorkspaceNotesQueryOptions(clients, workspaceId))
}

export async function prefetchWorkspaceBoards(queryClient: QueryClient, workspaceId: string) {
    const clients = await getWorkspaceQueryClients()
    return queryClient.prefetchQuery(makeWorkspaceBoardsQueryOptions(clients, workspaceId))
}