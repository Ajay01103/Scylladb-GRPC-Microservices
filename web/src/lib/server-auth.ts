import { cookies } from "next/headers"
import { redirect } from "next/navigation"

import { REFRESH_TOKEN_COOKIE_NAME } from "@/lib/auth-cookie"
import { validateRefreshToken } from "@/lib/server-jwt"

export async function hasServerSession() {
  // The cookie read is already request-scoped (Next.js renders layouts per
  // request when they call `cookies()`), so we don't need `unstable_noStore`.
  // Leaving it out keeps `requireAuthenticated` eligible for future
  // caching/memoization layers above this function.
  const cookieStore = await cookies()
  const refreshToken = cookieStore.get(REFRESH_TOKEN_COOKIE_NAME)?.value
  const validation = await validateRefreshToken(refreshToken)
  return validation.valid
}

export async function requireAuthenticated() {
  const authenticated = await hasServerSession()
  if (!authenticated) redirect("/login")
}

export async function requireGuest() {
  const authenticated = await hasServerSession()
  if (authenticated) redirect("/workspace")
}
