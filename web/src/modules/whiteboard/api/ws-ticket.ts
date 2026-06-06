"use client"

const WHITEBOARD_RPC_URL = process.env.NEXT_PUBLIC_WHITEBOARD_RPC_URL ?? "http://localhost:9093"

export async function requestWhiteboardWsTicket(
  accessToken: string,
  signal?: AbortSignal,
): Promise<string> {
  const response = await fetch(`${WHITEBOARD_RPC_URL}/ws/ticket`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
    signal,
  })

  if (!response.ok) {
    throw new Error("Failed to get websocket ticket")
  }

  const payload = (await response.json()) as { ticket?: string }
  if (!payload.ticket) {
    throw new Error("Websocket ticket response was empty")
  }

  return payload.ticket
}
