import { cookies } from "next/headers"
import { redirect } from "next/navigation"
import { unstable_noStore as noStore } from "next/cache"

import { REFRESH_TOKEN_COOKIE_NAME } from "@/lib/auth-cookie"
import { validateRefreshToken } from "@/lib/server-jwt"

export async function hasServerSession() {
  noStore()
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
  if (authenticated) redirect("/dashboard")
}
