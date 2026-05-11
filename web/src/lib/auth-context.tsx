"use client"

import { createContext, useContext, useEffect, useMemo, ReactNode } from "react"
import { tokenStore, useAccessToken, useIsLoadingAuth } from "@/lib/token-store"

interface AuthContextType {
  accessToken: string | null
  isAuthenticated: boolean
  isLoadingAuth: boolean
  setAccessToken: (token: string | null) => void
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const accessToken = useAccessToken()
  const isLoadingAuth = useIsLoadingAuth()

  useEffect(() => {
    tokenStore
      .refreshAccessTokenSingleton()
      .catch((err) => {
        console.error("Failed to restore session", err)
      })
      .finally(() => {
        tokenStore.setLoaded()
      })

    return () => {
      tokenStore.cancelRefreshTimer()
    }
  }, [])

  const value = useMemo(
    () => ({
      accessToken,
      isAuthenticated: accessToken !== null,
      isLoadingAuth,
      setAccessToken: tokenStore.set,
    }),
    [accessToken, isLoadingAuth],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) throw new Error("useAuth must be used within an AuthProvider")
  return context
}
