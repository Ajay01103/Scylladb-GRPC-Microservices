import { useQuery } from "@tanstack/react-query"
import { useAuth } from "@/lib/auth-context"
import { authBrowserRpcClient } from "@/lib/rpc"
import { useIsLoadingAuth } from "@/lib/token-store"

export type CurrentUser = {
  userId: string
  email: string
  name: string
}

export function useCurrentUser() {
  const { accessToken } = useAuth()
  const isLoadingAuth = useIsLoadingAuth()

  return useQuery({
    queryKey: ["currentUser", !!accessToken],
    enabled: !isLoadingAuth && !!accessToken,
    staleTime: 5 * 60 * 1000,
    gcTime: 30 * 60 * 1000,
    refetchOnWindowFocus: false,
    retry: 1,
    queryFn: async (): Promise<CurrentUser | null> => {
      const response = await authBrowserRpcClient.getCurrentUser({})

      return response as CurrentUser
    },
  })
}
