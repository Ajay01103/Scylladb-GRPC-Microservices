import { useMutation, useQueryClient } from "@tanstack/react-query"

import { logoutAction } from "@/actions/auth"
import { tokenStore } from "@/lib/token-store"
import { currentUserKey } from "./use-current-user"

export function useLogout() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async () => {
      // Server action reads the httpOnly cookie, calls RPC logout (revokes token
      // in Redis), then deletes the cookie — all in one server round-trip.
      await logoutAction()

      // Clear in-memory access token (and notify other tabs) and drop the
      // cached current-user query so the next sign-in refetches.
      tokenStore.reset()
      queryClient.removeQueries({ queryKey: currentUserKey })
    },
  })
}
