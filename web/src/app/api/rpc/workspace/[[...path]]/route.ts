import { createRpcProxy } from "@/lib/create-rpc-proxy"

const handler = createRpcProxy(
  process.env.WORKSPACE_RPC_URL ?? "http://localhost:9091",
  "/api/rpc/workspace",
)

export { handler as GET, handler as POST }
