"use client"

import { createClient } from "@connectrpc/connect"
import { createGrpcWebTransport } from "@connectrpc/connect-web"

import { AuthService } from "../gen/pb/auth/auth_pb"
import { NotesService } from "../gen/pb/notes/notes_pb"
import { WhiteboardService } from "../gen/pb/whiteboard/whiteboard_pb"
import { WorkspaceService } from "../gen/pb/workspace/workspace_pb"

// Same-origin proxy paths — browser calls Next.js, Next.js attaches Bearer
// from the HttpOnly cookie and forwards to the real Go service.
// No NEXT_PUBLIC_*_RPC_URL, no credentials:include, no token in JS.
const AUTH_BASE_URL = "/api/rpc/auth"
const NOTES_BASE_URL = "/api/rpc/notes"
const WHITEBOARD_BASE_URL = "/api/rpc/whiteboard"
const WORKSPACE_BASE_URL = "/api/rpc/workspace"

function createTransport(baseUrl: string) {
  // Go services use connectrpc with h2c — they accept grpc-web but not the
  // Connect protocol's application/connect+proto content type.
  // gRPC-Web works over HTTP/1.1 (what the browser → Next.js leg uses) and
  // the proxy forwards it unchanged to the Go service over HTTP/1.1 h2c.
  return createGrpcWebTransport({
    baseUrl,
    useBinaryFormat: true,
  })
}

// authBrowserRpcClient: authenticated calls (GetCurrentUser, etc.) via proxy.
// login/register/logout go through server actions, not this client.
export const authBrowserRpcClient = createClient(AuthService, createTransport(AUTH_BASE_URL))
export const notesRpcClient = createClient(NotesService, createTransport(NOTES_BASE_URL))
export const whiteboardRpcClient = createClient(
  WhiteboardService,
  createTransport(WHITEBOARD_BASE_URL),
)
export const workspaceRpcClient = createClient(
  WorkspaceService,
  createTransport(WORKSPACE_BASE_URL),
)
