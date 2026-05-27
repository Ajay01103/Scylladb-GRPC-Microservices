"use client"

import { useEffect, useMemo, useState } from "react"

import { Clock3, Copy, Link2, LogOut, Plus, RefreshCw, Shield, UserPlus, Users } from "lucide-react"
import { useRouter } from "next/navigation"

import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import {
  WorkspaceRole,
  type Workspace,
  type WorkspaceMember,
} from "@/gen/pb/workspace/workspace_pb"
import { useCurrentUser } from "@/modules/auth/api/use-current-user"
import {
  useGenerateWorkspaceInviteCode,
  useLeaveWorkspace,
} from "@/modules/workspace/api/use-workspaces"

import { WorkspaceMembersTable } from "./workspace-members-table"

function hasJoinedAt(member: WorkspaceMember) {
  return member.joinedAt != null
}

function isElevatedRole(member: WorkspaceMember) {
  const role = member.role as WorkspaceRole | string | number

  if (typeof role === "string") {
    return role === "WORKSPACE_ROLE_OWNER" || role === "WORKSPACE_ROLE_ADMIN"
  }

  return role === WorkspaceRole.OWNER || role === WorkspaceRole.ADMIN
}

function canManageMembers(role: WorkspaceRole | string | number) {
  if (typeof role === "string") {
    return role === "WORKSPACE_ROLE_OWNER" || role === "WORKSPACE_ROLE_ADMIN"
  }

  return role === WorkspaceRole.OWNER || role === WorkspaceRole.ADMIN
}

type WorkspaceMembersViewProps = {
  workspace?: Workspace
  members?: WorkspaceMember[]
  isMembersLoading?: boolean
}

export function WorkspaceMembersView({
  workspace,
  members = [],
  isMembersLoading = false,
}: WorkspaceMembersViewProps) {
  const router = useRouter()
  const { data: currentUser } = useCurrentUser()
  const inviteCodeMutation = useGenerateWorkspaceInviteCode()
  const leaveWorkspaceMutation = useLeaveWorkspace()

  const [isInviteDialogOpen, setIsInviteDialogOpen] = useState(false)
  const [inviteCode, setInviteCode] = useState("")
  const [copyState, setCopyState] = useState<"idle" | "copied" | "failed">("idle")

  const totalMembers = members.length
  const activeMembers = members.filter(hasJoinedAt).length
  const pendingInvites = members.filter((member) => !hasJoinedAt(member)).length
  const elevatedRoles = members.filter(isElevatedRole).length

  const isManager = workspace
    ? canManageMembers(workspace.myRole as WorkspaceRole | string | number)
    : false

  const inviteLink = useMemo(() => {
    if (!workspace || !inviteCode) {
      return ""
    }

    if (typeof window === "undefined") {
      return ""
    }

    const params = new URLSearchParams({
      inviteCode,
      workspaceId: workspace.id,
    })

    return `${window.location.origin}/workspace/invite?${params.toString()}`
  }, [inviteCode, workspace])

  useEffect(() => {
    if (!isInviteDialogOpen) {
      setCopyState("idle")
    }
  }, [isInviteDialogOpen])

  const generateInviteCode = async () => {
    if (!workspace) {
      return
    }

    const code = await inviteCodeMutation.mutateAsync({
      workspaceId: workspace.id,
      role: WorkspaceRole.MEMBER,
    })

    setInviteCode(code)
  }

  const handleOpenInviteDialog = async () => {
    setIsInviteDialogOpen(true)
    if (!inviteCode) {
      await generateInviteCode()
    }
  }

  const handleCopyInviteLink = async () => {
    if (!inviteLink) {
      return
    }

    try {
      await navigator.clipboard.writeText(inviteLink)
      setCopyState("copied")
    } catch {
      setCopyState("failed")
    }
  }

  const handleLeaveWorkspace = async () => {
    if (!workspace || !currentUser?.userId) {
      return
    }

    await leaveWorkspaceMutation.mutateAsync({
      workspaceId: workspace.id,
      userId: currentUser.userId,
    })

    router.replace("/workspace")
    router.refresh()
  }

  if (!workspace) {
    return (
      <div className="flex min-h-full flex-1 items-center justify-center p-6">
        <div className="w-full max-w-7xl space-y-4">
          <Skeleton className="h-[180px] w-full rounded-[32px]" />
          <Skeleton className="h-[120px] w-full rounded-2xl" />
          <Skeleton className="h-[420px] w-full rounded-3xl" />
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-full flex-1 bg-background">
      <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-4 sm:px-6 lg:px-8 lg:py-6">
        <section className="flex flex-col gap-4 rounded-[32px] border border-border/60 bg-card px-6 py-6 shadow-sm sm:px-8 lg:flex-row lg:items-end lg:justify-between">
          <div className="space-y-2">
            <p className="text-xs font-medium uppercase tracking-[0.26em] text-muted-foreground">
              Workspace members
            </p>
            <h1 className="text-3xl font-semibold tracking-tight text-foreground sm:text-4xl">
              {workspace.name || "Workspace"}
            </h1>
            <p className="max-w-2xl text-sm text-muted-foreground sm:text-base">
              Review everyone who has access to this workspace. Member data is loaded from the
              workspace service in real time.
            </p>
          </div>

          <div className="flex items-center gap-3">
            {isManager ? (
              <Button className="rounded-full px-5" onClick={handleOpenInviteDialog} size="sm">
                <Plus className="mr-2 size-4" />
                Invite member
              </Button>
            ) : (
              <Button
                className="rounded-full px-5"
                disabled={leaveWorkspaceMutation.isPending}
                onClick={handleLeaveWorkspace}
                size="sm"
                variant="outline"
              >
                <LogOut className="mr-2 size-4" />
                {leaveWorkspaceMutation.isPending ? "Leaving..." : "Leave workspace"}
              </Button>
            )}
          </div>
        </section>

        <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <div className="rounded-3xl border border-border/60 bg-card p-5 shadow-sm">
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-xs uppercase tracking-[0.22em] text-muted-foreground">
                  Total members
                </p>
                <p className="mt-3 text-3xl font-semibold text-foreground">{totalMembers}</p>
              </div>
              <div className="flex size-11 items-center justify-center rounded-2xl bg-muted text-muted-foreground">
                <Users className="size-5" />
              </div>
            </div>
          </div>

          <div className="rounded-3xl border border-border/60 bg-card p-5 shadow-sm">
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-xs uppercase tracking-[0.22em] text-muted-foreground">
                  Active members
                </p>
                <p className="mt-3 text-3xl font-semibold text-foreground">{activeMembers}</p>
              </div>
              <div className="flex size-11 items-center justify-center rounded-2xl bg-emerald-50 text-emerald-700">
                <UserPlus className="size-5" />
              </div>
            </div>
          </div>

          <div className="rounded-3xl border border-border/60 bg-card p-5 shadow-sm">
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-xs uppercase tracking-[0.22em] text-muted-foreground">
                  Pending invites
                </p>
                <p className="mt-3 text-3xl font-semibold text-foreground">{pendingInvites}</p>
              </div>
              <div className="flex size-11 items-center justify-center rounded-2xl bg-amber-50 text-amber-700">
                <Clock3 className="size-5" />
              </div>
            </div>
          </div>

          <div className="rounded-3xl border border-border/60 bg-card p-5 shadow-sm">
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-xs uppercase tracking-[0.22em] text-muted-foreground">
                  Elevated roles
                </p>
                <p className="mt-3 text-3xl font-semibold text-foreground">{elevatedRoles}</p>
              </div>
              <div className="flex size-11 items-center justify-center rounded-2xl bg-sky-50 text-sky-700">
                <Shield className="size-5" />
              </div>
            </div>
          </div>
        </section>

        <WorkspaceMembersTable isLoading={isMembersLoading} members={members} />

        <Dialog onOpenChange={setIsInviteDialogOpen} open={isInviteDialogOpen}>
          <DialogContent className="max-w-xl">
            <DialogHeader>
              <DialogTitle>Invite Member</DialogTitle>
              <DialogDescription>
                Share this invite link to let someone join this workspace as a member.
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-3">
              <label className="space-y-2">
                <span className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                  Invite link
                </span>
                <div className="flex items-center gap-2">
                  <div className="relative flex-1">
                    <Link2 className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
                    <Input className="pl-9" readOnly value={inviteLink} />
                  </div>
                </div>
              </label>

              {copyState === "copied" ? (
                <p className="text-xs text-emerald-700">Invite link copied.</p>
              ) : null}
              {copyState === "failed" ? (
                <p className="text-xs text-destructive">
                  Could not copy link. Please copy manually.
                </p>
              ) : null}
            </div>

            <DialogFooter className="gap-2">
              <Button
                disabled={inviteCodeMutation.isPending || !inviteLink}
                onClick={handleCopyInviteLink}
                variant="outline"
              >
                <Copy className="mr-2 size-4" />
                Copy link
              </Button>
              <Button disabled={inviteCodeMutation.isPending} onClick={generateInviteCode}>
                <RefreshCw className="mr-2 size-4" />
                {inviteCodeMutation.isPending ? "Resetting..." : "Reset link"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </div>
  )
}
