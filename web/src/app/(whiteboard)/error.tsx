"use client"

import { useEffect } from "react"

import { Button } from "@/components/ui/button"

export default function WhiteboardError({
  error,
  reset,
}: {
  error: Error & { digest?: string }
  reset: () => void
}) {
  useEffect(() => {
    console.error("Whiteboard error boundary:", error)
  }, [error])

  return (
    <div className="flex min-h-svh items-center justify-center p-6">
      <div className="max-w-md rounded-2xl border bg-card p-6 text-center shadow-sm">
        <p className="text-xs uppercase tracking-[0.2em] text-muted-foreground">Whiteboard error</p>
        <h1 className="mt-2 text-2xl font-semibold tracking-tight">Couldn’t load this board</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          {error.message || "An unexpected error occurred while loading the whiteboard."}
        </p>
        {error.digest ? (
          <p className="mt-2 text-xs text-muted-foreground">Error ID: {error.digest}</p>
        ) : null}
        <div className="mt-6 flex justify-center">
          <Button onClick={() => reset()}>Retry</Button>
        </div>
      </div>
    </div>
  )
}
