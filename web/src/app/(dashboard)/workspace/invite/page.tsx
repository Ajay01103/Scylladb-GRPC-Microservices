"use client"

import { useMemo, useState } from "react"

import { useRouter, useSearchParams } from "next/navigation"
import { Check, X } from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  useAcceptWorkspaceInvitation,
  useRejectWorkspaceInvitation,
} from "@/modules/workspace/api/use-workspaces"

function getInviteFailureMessage(error: unknown) {
  if (error instanceof Error) {
    const message = error.message.toLowerCase()

    if (message.includes("already used")) {
      return "This invite link was already used. Ask for a fresh invite link."
    }

    if (message.includes("expired")) {
      return "This invite link has expired. Ask for a fresh invite link."
    }

    if (message.includes("not found") || message.includes("invalid")) {
      return "This invite link is invalid. Ask for a valid invite link."
    }
  }

  return "Could not process this invite link. Please try again."
}

export default function WorkspaceInvitePage() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const acceptInviteMutation = useAcceptWorkspaceInvitation()
  const rejectInviteMutation = useRejectWorkspaceInvitation()
  const [statusMessage, setStatusMessage] = useState<string | null>(null)

  const inviteCode = useMemo(() => searchParams.get("inviteCode")?.trim() ?? "", [searchParams])
  const handleAccept = async () => {
    if (!inviteCode) {
      setStatusMessage("Invite code is missing from this link.")
      return
    }

    try {
      await acceptInviteMutation.mutateAsync(inviteCode)
      setStatusMessage("Invite accepted. Redirecting to workspace...")
      router.replace("/workspace")
      router.refresh()
    } catch (error) {
      setStatusMessage(getInviteFailureMessage(error))
    }
  }

  const handleReject = async () => {
    if (inviteCode) {
      try {
        await rejectInviteMutation.mutateAsync(inviteCode)
      } catch {
        // Ignore reject failures and still leave the invite page.
      }
    }

    router.replace("/workspace")
  }

  const hasValidParams = inviteCode.length > 0

  return (
    <div className="flex min-h-full flex-1 items-center justify-center p-6">
      <div className="w-full max-w-lg rounded-2xl border border-border/60 bg-card p-6 text-center shadow-sm">
        <p className="text-xs uppercase tracking-[0.2em] text-muted-foreground">Workspace invite</p>
        <h1 className="mt-2 text-2xl font-semibold tracking-tight">Join workspace</h1>
        <p className="mt-3 text-sm text-muted-foreground">
          {hasValidParams
            ? "You were invited to join this workspace. Choose to accept or reject this invite."
            : "Invite code is missing from this link."}
        </p>

        {statusMessage ? (
          <p className="mt-2 text-xs text-muted-foreground">{statusMessage}</p>
        ) : null}

        <div className="mt-5 flex items-center justify-center gap-3">
          <Button
            disabled={!hasValidParams || acceptInviteMutation.isPending}
            onClick={handleAccept}
          >
            <Check className="mr-2 size-4" />
            {acceptInviteMutation.isPending ? "Accepting..." : "Accept invite"}
          </Button>
          <Button
            disabled={acceptInviteMutation.isPending || rejectInviteMutation.isPending}
            onClick={handleReject}
            variant="outline"
          >
            <X className="mr-2 size-4" />
            {rejectInviteMutation.isPending ? "Rejecting..." : "Reject"}
          </Button>
        </div>
      </div>
    </div>
  )
}
