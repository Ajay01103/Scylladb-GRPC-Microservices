"use server"

import { createHash } from "node:crypto"
import { cookies } from "next/headers"
import { authRpcClient } from "@/lib/rpc"
import { REFRESH_TOKEN_COOKIE_NAME } from "@/lib/auth-cookie"

const serverInflightRefresh = new Map<string, Promise<string | null>>()

function refreshDedupKey(refreshToken: string): string {
  return createHash("sha256").update(refreshToken).digest("hex")
}

function setRefreshCookie(cookieStore: Awaited<ReturnType<typeof cookies>>, refreshToken: string) {
  cookieStore.set(REFRESH_TOKEN_COOKIE_NAME, refreshToken, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: 7 * 24 * 60 * 60,
  })
}

export async function setAuthCookies(refreshToken: string) {
  const cookieStore = await cookies()
  setRefreshCookie(cookieStore, refreshToken)
}

export async function refreshAccessTokenAction() {
  const cookieStore = await cookies()
  const refreshToken = cookieStore.get(REFRESH_TOKEN_COOKIE_NAME)?.value
  if (!refreshToken) return null

  const dedupKey = refreshDedupKey(refreshToken)
  const inFlight = serverInflightRefresh.get(dedupKey)
  if (inFlight) {
    return inFlight
  }

  const refreshPromise = (async () => {
    try {
      const response = await authRpcClient.refreshToken({
        refreshToken,
      })

      // Update the cookie with the newly issued refresh token
      setRefreshCookie(cookieStore, response.refreshToken)

      return response.accessToken
    } catch (error) {
      console.error("Failed to refresh token", error)
      // Refresh failure should only clear local session state.
      // Logout remains an explicit user action.
      cookieStore.delete(REFRESH_TOKEN_COOKIE_NAME)
      return null
    } finally {
      serverInflightRefresh.delete(dedupKey)
    }
  })()

  serverInflightRefresh.set(dedupKey, refreshPromise)
  return refreshPromise
}

export async function requireAccessTokenAction(): Promise<string> {
  const token = await refreshAccessTokenAction()
  if (!token) {
    throw new Error("Unauthorized")
  }

  return token
}

export async function logoutAction() {
  const cookieStore = await cookies()
  const refreshToken = cookieStore.get(REFRESH_TOKEN_COOKIE_NAME)?.value

  if (refreshToken) {
    try {
      // Call the gRPC endpoint to immediately revoke the token in Redis
      await authRpcClient.logout({ refreshToken })
    } catch (error) {
      // Log but don't block logout — still clear the cookie
      console.error("RPC logout failed", error)
    }
  }

  cookieStore.delete(REFRESH_TOKEN_COOKIE_NAME)
}
