"use client"

import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react"
import { tokenStore, useAccessToken, useIsLoadingAuth } from "@/lib/token-store"

interface AuthContextType {
  accessToken: string | null
  isAuthenticated: boolean
  isLoadingAuth: boolean
  authError: Error | null
  setAccessToken: (token: string | null) => void
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const accessToken = useAccessToken()
  const isLoadingAuth = useIsLoadingAuth()
  const [authError, setAuthError] = useState<Error | null>(null)

  useEffect(() => {
    tokenStore
      .refreshAccessTokenSingleton()
      .catch((err) => {
        setAuthError(err instanceof Error ? err : new Error(String(err)))
        console.error("Failed to restore session:", err)
      })

    return () => tokenStore.cancelRefreshTimer()
  }, [])

  const value = useMemo(
    () => ({
      accessToken,
      isAuthenticated: accessToken !== null,
      isLoadingAuth,
      authError,
      setAccessToken: tokenStore.set,
    }),
    [accessToken, isLoadingAuth, authError],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error("useAuth must be used within an AuthProvider")
  return ctx
}
