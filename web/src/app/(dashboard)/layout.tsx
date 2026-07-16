import { cookies } from "next/headers"
import { dehydrate, HydrationBoundary } from "@tanstack/react-query"

import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar"
import { DashboardSidebar } from "@/modules/dashboard/components/dashboard-sidebar"
import { requireAuthenticated } from "@/lib/server-auth"
import { getQueryClient } from "@/lib/get-query-client"
import { prefetchWorkspaces } from "@/modules/workspace/api/workspace-server-queries"

export default async function DashboardLayout({ children }: { children: React.ReactNode }) {
  await requireAuthenticated()

  const cookieStore = await cookies()
  const defaultOpen = cookieStore.get("sidebar_state")?.value === "true"

  // Prefetch the workspaces list so the sidebar switcher renders immediately
  // from dehydrated cache on first paint — no client-side RPC call on load.
  const queryClient = getQueryClient()
  await prefetchWorkspaces(queryClient)

  return (
    <HydrationBoundary state={dehydrate(queryClient)}>
      <SidebarProvider defaultOpen={defaultOpen} className="h-svh">
        <DashboardSidebar />
        <SidebarInset className="min-h-0 min-w-0">
          <main className="flex min-h-0 flex-1 flex-col">{children}</main>
        </SidebarInset>
      </SidebarProvider>
    </HydrationBoundary>
  )
}
