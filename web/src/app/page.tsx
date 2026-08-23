import { redirect } from "next/navigation"

import { getServerRpcClients } from "@/lib/rpc-server"

export default async function Home() {
  const { workspaceClient } = await getServerRpcClients()
  const { workspaces } = await workspaceClient.listMyWorkspaces({ pageSize: 1, pageToken: "" })

  if (workspaces.length === 0) {
    redirect("/workspace")
  }

  redirect(`/workspace/${workspaces[0].id}`)
}
