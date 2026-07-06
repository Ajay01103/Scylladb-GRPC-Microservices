"use client"

import { createContext, useContext, useMemo, type ReactNode } from "react"
import { useAuth } from "@/lib/auth-context"

const ANONYMOUS_USER = "anonymous"

type PollUserContextValue = {
  currentUser: string
  isAuthenticated: boolean
  isLoading: boolean
}

const PollUserContext = createContext<PollUserContextValue>({
  currentUser: ANONYMOUS_USER,
  isAuthenticated: false,
  isLoading: true,
})

export function usePollUser(): PollUserContextValue {
  return useContext(PollUserContext)
}

export function PollUserProvider({ children }: { children: ReactNode }) {
  // Single source of truth — no need to combine token + user query by hand.
  const { currentUser, isAuthenticated, isLoadingAuth, isLoadingUser } = useAuth()

  const value = useMemo(
    (): PollUserContextValue => ({
      currentUser: currentUser?.userId ?? ANONYMOUS_USER,
      isAuthenticated: isAuthenticated && Boolean(currentUser?.userId),
      isLoading: isLoadingAuth || (isAuthenticated && isLoadingUser),
    }),
    [currentUser?.userId, isAuthenticated, isLoadingAuth, isLoadingUser],
  )

  return <PollUserContext.Provider value={value}>{children}</PollUserContext.Provider>
}
