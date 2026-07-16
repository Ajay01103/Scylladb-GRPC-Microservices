import { redirect } from "next/navigation"

import { hasServerSession } from "@/lib/server-auth"

export default async function Home() {
  const authenticated = await hasServerSession()

  if (!authenticated) {
    redirect("/login")
  }

  redirect("/redirect")
}
