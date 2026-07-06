"use client"

import { queryOptions, useQuery } from "@tanstack/react-query"
import { useSyncExternalStore } from "react"

import { authBrowserRpcClient } from "@/lib/rpc"
import { tokenStore } from "@/lib/token-store"

export type CurrentUser = {
  userId: string
  email: string
  name: string
}

/**
 * Single source-of-truth query key for the current user. Use this with
 * `queryClient.invalidateQueries({ queryKey: currentUserKey })` /
 * `queryClient.removeQueries({ queryKey: currentUserKey })` everywhere so
 * logout flows don't have to hard-code the array.
 */
export const currentUserKey = ["currentUser"] as const

export function currentUserOptions() {
  return queryOptions({
    queryKey: currentUserKey,
    queryFn: async (): Promise<CurrentUser> => {
      // The proto response is structurally compatible with CurrentUser today.
      // If the schema diverges, transform here instead of casting at the call
      // site so every consumer gets a well-typed DTO.
      const response = await authBrowserRpcClient.getCurrentUser({})
      return {
        userId: response.userId,
        email: response.email,
        name: response.name,
      }
    },
    staleTime: 5 * 60 * 1000,
    gcTime: 30 * 60 * 1000,
    refetchOnWindowFocus: false,
    retry: 1,
  })
}

type UseCurrentUserOptions = {
  /** Override the auth gate. Defaults to "only run when authenticated". */
  enabled?: boolean
}

/**
 * Returns the current user's profile. The query is gated on the in-memory
 * access token, so it's idle while signed out and flips to fetching the
 * moment a token is set.
 */
export function useCurrentUser(options: UseCurrentUserOptions = {}) {
  const { enabled: callerEnabled = true } = options

  // Subscribe to the token store directly (not via `useAuth`) to avoid a
  // circular dep: useAuth composes useCurrentUser.
  const hasToken = useSyncExternalStore(
    tokenStore.subscribe,
    () => tokenStore.getSnapshot() !== null,
    () => false,
  )

  return useQuery({
    ...currentUserOptions(),
    enabled: hasToken && callerEnabled,
  })
}
