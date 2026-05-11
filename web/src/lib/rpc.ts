import { createClient, type Interceptor } from "@connectrpc/connect"
import { createConnectTransport } from "@connectrpc/connect-web"
import { tokenStore } from "@/lib/token-store"

import { AuthService } from "../gen/pb/auth_pb"

const authBaseUrl = process.env.NEXT_PUBLIC_AUTH_RPC_URL ?? "http://localhost:50051"

const bearerAuthInterceptor: Interceptor = (next) => async (req) => {
  const token = typeof tokenStore.get === "function" ? tokenStore.get() : null
  if (token) {
    req.header.set("Authorization", `Bearer ${token}`)
  }

  return next(req)
}

const authTransport = createConnectTransport({
  baseUrl: authBaseUrl,
  useBinaryFormat: true,
})

const browserAuthTransport = createConnectTransport({
  baseUrl: authBaseUrl,
  interceptors: [bearerAuthInterceptor],
  useBinaryFormat: true,
})

export const authRpcClient = createClient(AuthService, authTransport)
export const authBrowserRpcClient = createClient(AuthService, browserAuthTransport)
