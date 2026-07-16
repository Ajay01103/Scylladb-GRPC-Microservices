import { redirect } from "next/navigation"

import { hasServerSession } from "@/lib/server-auth"
import { getServerRpcClients } from "@/lib/rpc-server"

export const dynamic = "force-dynamic"

export async function GET() {
  const authenticated = await hasServerSession()

  if (!authenticated) {
    redirect("/login")
  }

  const { workspaceClient } = await getServerRpcClients()
  const { workspaces } = await workspaceClient.listMyWorkspaces({ pageSize: 1, pageToken: "" })

  if (workspaces.length === 0) {
    redirect("/workspace")
  }

  redirect(`/workspace/${workspaces[0].id}`)
}
