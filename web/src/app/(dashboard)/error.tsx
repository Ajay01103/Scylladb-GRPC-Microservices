"use client"

import { useEffect } from "react"
import { useRouter } from "next/navigation"

import { Button } from "@/components/ui/button"

// Route-group-scoped error boundary. Wraps every page under (dashboard)
// so a single render error (e.g. RPC failure, hydration mismatch) shows a
// recoverable error UI instead of a blank screen.

export default function DashboardError({
  error,
  reset,
}: {
  error: Error & { digest?: string }
  reset: () => void
}) {
  const router = useRouter()

  useEffect(() => {
    console.error("Dashboard error boundary:", error)
  }, [error])

  return (
    <div className="flex min-h-svh items-center justify-center p-6">
      <div className="max-w-md rounded-2xl border bg-card p-6 text-center shadow-sm">
        <p className="text-xs uppercase tracking-[0.2em] text-muted-foreground">Error</p>
        <h1 className="mt-2 text-2xl font-semibold tracking-tight">Something went wrong</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          {error.message || "An unexpected error occurred."}
        </p>
        {error.digest ? (
          <p className="mt-2 text-xs text-muted-foreground">Error ID: {error.digest}</p>
        ) : null}
        <div className="mt-6 flex justify-center gap-2">
          <Button variant="outline" onClick={() => reset()}>
            Try again
          </Button>
          <Button onClick={() => router.refresh()}>Reload</Button>
        </div>
      </div>
    </div>
  )
}
