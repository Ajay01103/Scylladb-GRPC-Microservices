"use client"

import {
  QueryClient,
  QueryClientProvider,
  defaultShouldDehydrateQuery,
  isServer,
} from "@tanstack/react-query"
import { useState } from "react"

export function makeQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        // Data is fresh for 60s — no refetch on re-mount within that window
        staleTime: 60 * 1000,
        // Keep inactive cache entries for 5 minutes before GC
        gcTime: 5 * 60 * 1000,
        // Only retry once on failure (default 3 is too noisy for auth errors)
        retry: 1,
        // Don't refetch on window focus in most cases — opt-in per-query if needed
        refetchOnWindowFocus: false,
      },
      mutations: {
        // Surface mutation errors to nearest error boundary
        throwOnError: false,
        retry: 0,
      },
      dehydrate: {
        // Include pending (in-flight) queries in SSR dehydration payload
        // Required if you prefetchQuery in Server Components
        shouldDehydrateQuery: (query) =>
          defaultShouldDehydrateQuery(query) || query.state.status === "pending",
      },
    },
  })
}

// Module-level singleton ONLY for the server — safe because each
// SSR request runs in its own async context via AsyncLocalStorage.
// On the client this is never used.
let browserQueryClient: QueryClient | undefined = undefined

function getQueryClient() {
  if (isServer) {
    // Server: always make a new client to avoid cross-request data leaking
    return makeQueryClient()
  }

  // Browser: reuse existing client or create once
  if (!browserQueryClient) {
    browserQueryClient = makeQueryClient()
  }

  return browserQueryClient
}

export function QueryProvider({ children }: { children: React.ReactNode }) {
  // useState with no initial value + getQueryClient() avoids the client
  // being recreated on every render while still being safe for Suspense
  // (unlike a plain module-level variable on the client).
  const [queryClient] = useState(getQueryClient)

  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
}
