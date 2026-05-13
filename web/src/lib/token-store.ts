"use client"

import { refreshAccessTokenAction } from "@/actions/auth"
import { useSyncExternalStore } from "react"

type Listener = () => void

let accessToken: string | null = null
let isLoadingAuth = true
let refreshTimer: ReturnType<typeof setTimeout> | null = null
let inFlightRefresh: Promise<string | null> | null = null
const listeners = new Set<Listener>()

function emit() {
  listeners.forEach((listener) => listener())
}

function parseJwtExp(token: string): number | null {
  try {
    const payload = JSON.parse(atob(token.split(".")[1]))
    return payload.exp ?? null
  } catch {
    return null
  }
}

function cancelRefreshTimer() {
  if (!refreshTimer) return
  clearTimeout(refreshTimer)
  refreshTimer = null
}

export const tokenStore = {
  get: () => accessToken,
  set: (token: string | null) => {
    if (accessToken === token) return

    accessToken = token
    cancelRefreshTimer()

    if (token) {
      const exp = parseJwtExp(token)
      if (exp) {
        const delay = Math.max(exp * 1000 - Date.now() - 30_000, 0)
        refreshTimer = setTimeout(() => {
          void tokenStore.refreshAccessTokenSingleton()
        }, delay)
      }
    }

    emit()
  },
  refreshAccessTokenSingleton: (): Promise<string | null> => {
    if (inFlightRefresh) {
      return inFlightRefresh
    }

    // Prevent an already-scheduled timer callback from launching a second refresh.
    cancelRefreshTimer()

    inFlightRefresh = (async () => {
      try {
        const token = await refreshAccessTokenAction()
        tokenStore.set(token)
        return token
      } finally {
        inFlightRefresh = null
      }
    })()

    return inFlightRefresh
  },
  subscribe: (listener: Listener) => {
    listeners.add(listener)
    return () => listeners.delete(listener)
  },
  getSnapshot: () => accessToken,
  setLoaded: () => {
    if (!isLoadingAuth) return
    isLoadingAuth = false
    emit()
  },
  getIsLoading: () => isLoadingAuth,
  cancelRefreshTimer,
}

export function useAccessToken() {
  return useSyncExternalStore(tokenStore.subscribe, tokenStore.getSnapshot, () => null)
}

export function useIsLoadingAuth() {
  return useSyncExternalStore(tokenStore.subscribe, tokenStore.getIsLoading, () => true)
}
