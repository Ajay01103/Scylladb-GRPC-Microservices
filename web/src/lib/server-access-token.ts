import "server-only"

import { cache } from "react"
import { cookies } from "next/headers"
import { ACCESS_TOKEN_COOKIE_NAME } from "@/lib/auth-cookie"

/**
 * Per-request memoised access token READ.
 *
 * No refresh, no cookie mutation happens here — that's handled
 * entirely by middleware.ts before this render ever starts, since
 * Server Components are only permitted to READ cookies, never write
 * them (Server Actions / Route Handlers only).
 *
 * By the time a Server Component runs, middleware has already
 * guaranteed a fresh access token cookie is present (or redirected
 * to /login if the session couldn't be refreshed at all).
 */
async function readAccessTokenFromCookie(): Promise<string | null> {
  const store = await cookies()
  return store.get(ACCESS_TOKEN_COOKIE_NAME)?.value ?? null
}

export const getServerAccessToken = cache(readAccessTokenFromCookie)

// import "server-only"

// import { createHash } from "node:crypto"
// import { cache } from "react"
// import { cookies } from "next/headers"
// import { createClient } from "@connectrpc/connect"
// import { createGrpcTransport } from "@connectrpc/connect-node" // ✅ node transport
// import { AuthService } from "@/gen/pb/auth/auth_pb"
// import { REFRESH_TOKEN_COOKIE_NAME } from "@/lib/auth-cookie"

// // ✅ Standalone — never imports from @/lib/rpc (client bundle)
// // ✅ Private env var — never reaches the browser
// const _internalAuthTransport = createGrpcTransport({
//   baseUrl: process.env.AUTH_RPC_URL ?? "http://localhost:50051",
//   // httpVersion: "2",
// })

// const _internalAuthClient = createClient(AuthService, _internalAuthTransport)

// // ─────────────────────────────────────────────
// // Module-level map for cross-request dedup
// // (cache() handles within-request dedup)
// // ─────────────────────────────────────────────
// const serverInflightRefresh = new Map<string, Promise<string | null>>()

// function refreshDedupKey(token: string): string {
//   return createHash("sha256").update(token).digest("hex")
// }

// function setRefreshCookie(
//   store: Awaited<ReturnType<typeof cookies>>,
//   refreshToken: string,
// ) {
//   store.set(REFRESH_TOKEN_COOKIE_NAME, refreshToken, {
//     httpOnly: true,
//     secure: process.env.NODE_ENV === "production",
//     sameSite: "lax",
//     path: "/",
//     maxAge: 7 * 24 * 60 * 60,
//   })
// }

// async function refreshAccessTokenFromCookie(): Promise<string | null> {
//   const store = await cookies()
//   const refreshToken = store.get(REFRESH_TOKEN_COOKIE_NAME)?.value
//   if (!refreshToken) return null

//   const key = refreshDedupKey(refreshToken)
//   const inFlight = serverInflightRefresh.get(key)
//   if (inFlight) return inFlight

//   const promise = (async () => {
//     try {
//       const res = await _internalAuthClient.refreshToken({ refreshToken })
//       setRefreshCookie(store, res.refreshToken)
//       return res.accessToken        // EdDSA-signed JWT — opaque to us, backend verifies
//     } catch (err) {
//       console.error("[server-access-token] refresh failed:", err)
//       store.delete(REFRESH_TOKEN_COOKIE_NAME)
//       return null
//     } finally {
//       serverInflightRefresh.delete(key)
//     }
//   })()

//   serverInflightRefresh.set(key, promise)
//   return promise
// }

// /**
//  * Per-request memoised access token.
//  * Safe for Server Components and prefetch functions.
//  * EdDSA tokens are passed through as opaque strings — backend owns verification.
//  */
// export const getServerAccessToken = cache(refreshAccessTokenFromCookie)
