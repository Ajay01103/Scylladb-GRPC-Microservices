"use client"

// Single source of truth for client-side auth state.
//
// `useAuth()` is the only hook app code should call for auth. It merges:
//   - the imperative `tokenStore` (via `useSyncExternalStore`)
//   - the `currentUser` TanStack Query (via `useCurrentUser`)
//
// `AuthProvider` is a pure side-effect component: it kicks off the initial
// silent refresh on mount. No React Context is involved \u2014 every consumer
// reads the same external store state, so there is no chance of a stale
// Provider value diverging from reality.

import { useEffect, useMemo, useSyncExternalStore, type ReactNode } from "react"

import type { CurrentUser } from "@/modules/auth/api/use-current-user"
import { useCurrentUser } from "@/modules/auth/api/use-current-user"
import { tokenStore } from "@/lib/token-store"

export interface AuthState {
  /** Raw in-memory JWT, or null when signed out / still restoring. */
  accessToken: string | null
  /** True once the in-memory token is present. */
  isAuthenticated: boolean
  /**
   * True until the first refresh attempt has resolved (success OR failure).
   * Use this to gate protected queries and avoid flicker on first paint.
   */
  isLoadingAuth: boolean
  /** Profile returned by the backend, fetched only when authenticated. */
  currentUser: CurrentUser | null
  /** True while the current-user query is in flight. */
  isLoadingUser: boolean
  /** Imperative setter used by login flows. */
  setAccessToken: (token: string | null) => void
}

function useAccessToken() {
  return useSyncExternalStore(
    tokenStore.subscribe,
    tokenStore.getSnapshot,
    () => null,
  )
}

function useIsLoadingAuth() {
  return useSyncExternalStore(
    tokenStore.subscribe,
    tokenStore.getIsLoading,
    () => true,
  )
}

/**
 * Single auth hook. Subscribes to the token store and merges the
 * `currentUser` query. Prefer this over calling `useCurrentUser` /
 * `useIsLoadingAuth` directly \u2014 that keeps every consumer on one shape.
 */
export function useAuth(): AuthState {
  const accessToken = useAccessToken()
  const isLoadingAuth = useIsLoadingAuth()

  // `enabled` keeps the query idle until we have a token, so the auth gate
  // and the user query stay in lockstep \u2014 no double-source-of-truth drift.
  const userQuery = useCurrentUser({ enabled: !!accessToken })

  return useMemo<AuthState>(
    () => ({
      accessToken,
      isAuthenticated: accessToken !== null,
      isLoadingAuth,
      currentUser: userQuery.data ?? null,
      isLoadingUser: userQuery.isLoading,
      setAccessToken: tokenStore.set,
    }),
    [accessToken, isLoadingAuth, userQuery.data, userQuery.isLoading],
  )
}

/**
 * Mount once near the root of the client tree. Its only job is to kick off
 * the initial silent refresh so `isLoadingAuth` flips to `false` and gated
 * queries can run. Failures are written back into `tokenStore` so the next
 * `useAuth()` call sees them.
 */
export function AuthProvider({ children }: { children: ReactNode }) {
  useEffect(() => {
    tokenStore
      .refreshAccessTokenSingleton()
      .catch((err) => {
        const e = err instanceof Error ? err : new Error(String(err))
        console.error("Failed to restore session:", e)
      })
  }, [])

  return <>{children}</>
}
