import { requireGuest } from "@/lib/server-auth"

export default async function AuthLayout({ children }: { children: React.ReactNode }) {
  await requireGuest()

  return children
}
