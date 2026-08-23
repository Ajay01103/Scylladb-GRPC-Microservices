import { cookies } from "next/headers"
import { dehydrate, HydrationBoundary } from "@tanstack/react-query"

import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar"
import { DashboardSidebar } from "@/modules/dashboard/components/dashboard-sidebar"
import { getQueryClient } from "@/lib/get-query-client"
import { prefetchWorkspaces } from "@/modules/workspace/api/workspace-server-queries"

export default async function DashboardLayout({ children }: { children: React.ReactNode }) {
  const cookieStore = await cookies()
  const defaultOpen = cookieStore.get("sidebar_state")?.value === "true"

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
