"use client"

const NOTES_RPC_URL = process.env.NEXT_PUBLIC_NOTES_RPC_URL ?? "http://localhost:9092"

/**
 * Fetches a short-lived one-time WebSocket ticket from the notes service.
 * The ticket is then passed as `?ticket=` when opening the WS connection,
 * because browsers cannot send custom Authorization headers over WebSocket.
 */
export async function requestNotesWsTicket(
  accessToken: string,
  signal?: AbortSignal,
): Promise<string> {
  const response = await fetch(`${NOTES_RPC_URL}/ws/ticket`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
    signal,
  })

  if (!response.ok) {
    throw new Error(`Failed to get notes websocket ticket: ${response.status}`)
  }

  const payload = (await response.json()) as { ticket?: string }
  if (!payload.ticket) {
    throw new Error("Notes websocket ticket response was empty")
  }

  return payload.ticket
}
