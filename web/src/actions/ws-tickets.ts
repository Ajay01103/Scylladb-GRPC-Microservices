"use server"

import { getServerAccessToken } from "@/lib/server-access-token"

const NOTES_RPC_URL = process.env.NOTES_RPC_URL ?? "http://localhost:9092"
const WHITEBOARD_RPC_URL = process.env.WHITEBOARD_RPC_URL ?? "http://localhost:9093"

async function fetchWsTicket(url: string): Promise<string> {
  const token = await getServerAccessToken()
  if (!token) {
    throw new Error("No access token — user is not authenticated")
  }

  const response = await fetch(url, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  })

  if (!response.ok) {
    throw new Error(`Failed to get WS ticket from ${url}: ${response.status}`)
  }

  const payload = (await response.json()) as { ticket?: string }
  if (!payload.ticket) {
    throw new Error("WS ticket response was empty")
  }

  return payload.ticket
}

/**
 * Fetches a short-lived WebSocket ticket for the notes service.
 * Reads the access token from the HttpOnly cookie — never exposes it to JS.
 */
export async function requestNotesWsTicketAction(): Promise<string> {
  return fetchWsTicket(`${NOTES_RPC_URL}/ws/ticket`)
}

/**
 * Fetches a short-lived WebSocket ticket for the whiteboard service.
 * Reads the access token from the HttpOnly cookie — never exposes it to JS.
 */
export async function requestWhiteboardWsTicketAction(): Promise<string> {
  return fetchWsTicket(`${WHITEBOARD_RPC_URL}/ws/ticket`)
}
