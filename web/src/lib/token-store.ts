"use client"

import { refreshAccessTokenAction } from "@/actions/auth"
import { useSyncExternalStore } from "react"

type Listener = () => void

let accessToken: string | null = null
let initialized = false
let refreshTimer: ReturnType<typeof setTimeout> | null = null
let inFlightRefresh: Promise<string | null> | null = null
const listeners = new Set<Listener>()
let browserListenersAttached = false

const emit = () => listeners.forEach((l) => l())

// Cross-tab channel for logout notifications
const channel =
  typeof window !== "undefined" && "BroadcastChannel" in window
    ? new BroadcastChannel("motion_auth")
    : null

function parseJwtExp(token: string): number | null {
  try {
    const part = token.split(".")[1]
    if (!part) return null
    // base64url -> base64
    const padded = part.replace(/-/g, "+").replace(/_/g, "/")
    const pad = (4 - (padded.length % 4)) % 4
    const withPad = padded + "=".repeat(pad)
    const payload = JSON.parse(atob(withPad))
    return typeof payload.exp === "number" ? payload.exp : null
  } catch {
    return null
  }
}

function isExpiringSoon(token: string | null): boolean {
  if (!token) return true
  const exp = parseJwtExp(token)
  return !exp || exp * 1000 - Date.now() <= 30_000
}

function scheduleRefresh(token: string) {
  const exp = parseJwtExp(token)
  if (!exp) return
  const delay = Math.max(exp * 1000 - Date.now() - 30_000, 0)
  refreshTimer = setTimeout(() => void tokenStore.refreshAccessTokenSingleton(), delay)
}

function attachBrowserListeners() {
  if (browserListenersAttached || typeof window === "undefined") {
    return
  }

  browserListenersAttached = true

  channel?.addEventListener("message", (e) => {
    try {
      if (e.data?.type === "logout") {
        accessToken = null
        // Mark initialized so we don't re-trigger loading in other tabs.
        initialized = true
        tokenStore.cancelRefreshTimer()
        emit()
      }
    } catch {
      // Ignore malformed broadcast payloads.
    }
  })

  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "visible" && isExpiringSoon(accessToken)) {
      void tokenStore.refreshAccessTokenSingleton()
    }
  })
}

if (typeof window !== "undefined") {
  attachBrowserListeners()
}

export const tokenStore = {
  getSnapshot: () => accessToken,
  getIsLoading: () => !initialized,

  subscribe(listener: Listener) {
    listeners.add(listener)
    return () => listeners.delete(listener)
  },

  set(token: string | null) {
    if (accessToken === token) return
    accessToken = token
    initialized = true
    tokenStore.cancelRefreshTimer()
    if (token) scheduleRefresh(token)
    emit()
  },

  // Reset store state on logout across tabs
  reset() {
    accessToken = null
    initialized = false
    tokenStore.cancelRefreshTimer()
    try {
      channel?.postMessage({ type: "logout" })
    } catch {}
    emit()
  },

  setInitialized() {
    if (initialized) return
    initialized = true
    emit()
  },

  cancelRefreshTimer() {
    if (!refreshTimer) return
    clearTimeout(refreshTimer)
    refreshTimer = null
  },

  refreshAccessTokenSingleton(): Promise<string | null> {
    if (inFlightRefresh) return inFlightRefresh

    tokenStore.cancelRefreshTimer()

    inFlightRefresh = refreshAccessTokenAction()
      .then((token) => {
        // Set token and mark initialized together to avoid double-emits
        accessToken = token
        initialized = true
        tokenStore.cancelRefreshTimer()
        if (token) scheduleRefresh(token)
        emit()
        return token
      })
      .catch((err) => {
        // Even on failure, mark initialized so UI stops loading state
        initialized = true
        emit()
        throw err
      })
      .finally(() => {
        inFlightRefresh = null
      })

    return inFlightRefresh
  },

  async ensureValidAccessToken(): Promise<string | null> {
    if (!initialized || isExpiringSoon(accessToken)) {
      return tokenStore.refreshAccessTokenSingleton()
    }
    return accessToken
  },
}

export function useAccessToken() {
  return useSyncExternalStore(tokenStore.subscribe, tokenStore.getSnapshot, () => null)
}

export function useIsLoadingAuth() {
  return useSyncExternalStore(tokenStore.subscribe, tokenStore.getIsLoading, () => true)
}
