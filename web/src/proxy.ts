import type { NextRequest } from "next/server"
import { NextResponse } from "next/server"

import { REFRESH_TOKEN_COOKIE_NAME } from "@/lib/auth-cookie"
import { validateRefreshToken } from "@/lib/server-jwt"

function redirectToLogin(request: NextRequest) {
  return NextResponse.redirect(new URL("/login", request.url))
}

export async function proxy(request: NextRequest) {
  const refreshToken = request.cookies.get(REFRESH_TOKEN_COOKIE_NAME)?.value
  const validation = await validateRefreshToken(refreshToken)
  if (!validation.valid) {
    return redirectToLogin(request)
  }

  return NextResponse.next()
}

export const config = {
  matcher: ["/dashboard/:path*"],
}
