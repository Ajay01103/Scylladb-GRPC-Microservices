import Link from "next/link"

import { Button } from "@/components/ui/button"

export default function DashboardNotFound() {
  return (
    <div className="flex min-h-svh items-center justify-center p-6">
      <div className="max-w-md rounded-2xl border border-dashed bg-card p-6 text-center shadow-sm">
        <p className="text-xs uppercase tracking-[0.2em] text-muted-foreground">404</p>
        <h1 className="mt-2 text-2xl font-semibold tracking-tight">Page not found</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          The page or workspace you’re looking for doesn’t exist or was deleted.
        </p>
        <div className="mt-6 flex justify-center">
          <Button asChild>
            <Link href="/workspace">Back to workspaces</Link>
          </Button>
        </div>
      </div>
    </div>
  )
}
