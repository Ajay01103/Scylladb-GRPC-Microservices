import { redirect } from "next/navigation"

import { hasServerSession } from "@/lib/server-auth"

export default async function Home() {
  const authenticated = await hasServerSession()

  redirect(authenticated ? "/workspace" : "/login")
}
