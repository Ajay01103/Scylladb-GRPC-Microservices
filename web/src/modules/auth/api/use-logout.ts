import { useMutation, useQueryClient } from "@tanstack/react-query"
import { tokenStore } from "@/lib/token-store"
import { logoutAction } from "@/actions/auth"

export function useLogout() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async () => {
      // Server action reads the httpOnly cookie, calls RPC logout (revokes token
      // in Redis), then deletes the cookie — all in one server round-trip.
      await logoutAction()

      // Clear in-memory access token (and notify other tabs) and invalidate user query
      tokenStore.reset()
      queryClient.removeQueries({ queryKey: ["currentUser"] })
    },
  })
}
