import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useAuth } from "@/lib/auth-context"
import { logoutAction } from "@/actions/auth"

export function useLogout() {
  const { setAccessToken } = useAuth()
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async () => {
      // Server action reads the httpOnly cookie, calls RPC logout (revokes token
      // in Redis), then deletes the cookie — all in one server round-trip.
      await logoutAction()

      // Clear in-memory access token and invalidate user query
      setAccessToken(null)
      queryClient.removeQueries({ queryKey: ["currentUser"] })
    },
  })
}
