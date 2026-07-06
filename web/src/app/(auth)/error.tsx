"use client"

import { useEffect } from "react"

import { Button } from "@/components/ui/button"

export default function AuthError({
  error,
  reset,
}: {
  error: Error & { digest?: string }
  reset: () => void
}) {
  useEffect(() => {
    console.error("Auth error boundary:", error)
  }, [error])

  return (
    <div className="flex min-h-svh items-center justify-center p-6">
      <div className="max-w-md rounded-2xl border bg-card p-6 text-center shadow-sm">
        <p className="text-xs uppercase tracking-[0.2em] text-muted-foreground">
          Sign-in error
        </p>
        <h1 className="mt-2 text-2xl font-semibold tracking-tight">
          Something went wrong
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">
          {error.message || "We couldn’t complete that request."}
        </p>
        {error.digest ? (
          <p className="mt-2 text-xs text-muted-foreground">Error ID: {error.digest}</p>
        ) : null}
        <div className="mt-6 flex justify-center">
          <Button onClick={() => reset()}>Try again</Button>
        </div>
      </div>
    </div>
  )
}
