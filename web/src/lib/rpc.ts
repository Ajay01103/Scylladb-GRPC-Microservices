import { Code, ConnectError, createClient, type Interceptor } from "@connectrpc/connect"
import { createConnectTransport } from "@connectrpc/connect-web"
import { tokenStore } from "@/lib/token-store"
import { AuthService } from "../gen/pb/auth/auth_pb"
import { WorkspaceService } from "../gen/pb/workspace/workspace_pb"

const AUTH_BASE_URL = process.env.NEXT_PUBLIC_AUTH_RPC_URL ?? "http://localhost:50051"
const WORKSPACE_BASE_URL = process.env.NEXT_PUBLIC_WORKSPACE_RPC_URL ?? "http://localhost:9091"

const authInterceptor: Interceptor = (next) => async (req) => {
  const token = await tokenStore.ensureValidAccessToken()
  if (token) req.header.set("Authorization", `Bearer ${token}`)

  try {
    return await next(req)
  } catch (error) {
    if (error instanceof ConnectError && error.code === Code.Unauthenticated) {
      const refreshed = await tokenStore.refreshAccessTokenSingleton()
      if (refreshed) {
        req.header.set("Authorization", `Bearer ${refreshed}`)
        return next(req)
      }
    }
    throw error
  }
}

function createTransport(baseUrl: string, authenticated = false) {
  return createConnectTransport({
    baseUrl,
    useBinaryFormat: true,
    ...(authenticated && { interceptors: [authInterceptor] }),
  })
}

// Unauthenticated — for login / register
export const authRpcClient = createClient(AuthService, createTransport(AUTH_BASE_URL))

// Authenticated — for logout, profile, etc.
export const authBrowserRpcClient = createClient(AuthService, createTransport(AUTH_BASE_URL, true))

// Always authenticated
export const workspaceRpcClient = createClient(WorkspaceService, createTransport(WORKSPACE_BASE_URL, true))