import { createRpcProxy } from "@/lib/create-rpc-proxy"

const handler = createRpcProxy(
  process.env.NOTES_RPC_URL ?? "http://localhost:9092",
  "/api/rpc/notes",
)

export { handler as GET, handler as POST }
