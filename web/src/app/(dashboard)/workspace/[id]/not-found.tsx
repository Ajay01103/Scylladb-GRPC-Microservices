import Link from "next/link"

import { Button } from "@/components/ui/button"

export default function WorkspaceNotFound() {
  return (
    <div className="flex min-h-full flex-1 items-center justify-center p-6">
      <div className="max-w-md rounded-2xl border border-dashed bg-card p-6 text-center shadow-sm">
        <p className="text-xs uppercase tracking-[0.2em] text-muted-foreground">Workspace</p>
        <h1 className="mt-2 text-2xl font-semibold tracking-tight">Workspace not found</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          This workspace may have been deleted, or you may not have access to it.
        </p>
        <div className="mt-6 flex justify-center">
          <Button asChild>
            <Link href="/workspace">Go to your workspaces</Link>
          </Button>
        </div>
      </div>
    </div>
  )
}
