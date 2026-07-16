import { createRpcProxy } from "@/lib/create-rpc-proxy"

const handler = createRpcProxy(
  process.env.WHITEBOARD_RPC_URL ?? "http://localhost:9093",
  "/api/rpc/whiteboard",
)

export { handler as GET, handler as POST }
