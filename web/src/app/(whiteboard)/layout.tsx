import { requireAuthenticated } from "@/lib/server-auth"

export default async function WhiteboardLayout({ children }: { children: React.ReactNode }) {
  await requireAuthenticated()

  return <main className="min-h-svh w-full">{children}</main>
}
