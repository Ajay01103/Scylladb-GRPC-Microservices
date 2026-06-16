"use client"

import { createContext, useContext, useMemo, type ReactNode } from "react"
import { useAuth } from "@/lib/auth-context"
import { useIsLoadingAuth } from "@/lib/token-store"
import { useCurrentUser } from "@/modules/auth/api/use-current-user"

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
  const { isAuthenticated, isLoadingAuth } = useAuth()
  const isLoadingToken = useIsLoadingAuth()
  const { data: user, isLoading: isLoadingUser } = useCurrentUser()

  const value = useMemo(
    (): PollUserContextValue => ({
      currentUser: user?.userId ?? ANONYMOUS_USER,
      isAuthenticated: isAuthenticated && Boolean(user?.userId),
      isLoading:
        isLoadingAuth || isLoadingToken || (isAuthenticated && isLoadingUser),
    }),
    [
      user?.userId,
      isAuthenticated,
      isLoadingAuth,
      isLoadingToken,
      isLoadingUser,
    ],
  )

  return (
    <PollUserContext.Provider value={value}>{children}</PollUserContext.Provider>
  )
}
