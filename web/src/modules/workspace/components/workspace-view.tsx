"use client"

import { useMemo, useState } from "react"

import { Plus } from "lucide-react"
import { useRouter } from "next/navigation"

import AnimatedTabs from "@/components/ui/animated-tabs"
import { Button } from "@/components/ui/button"
import type { Workspace } from "@/gen/pb/workspace/workspace_pb"
import { Skeleton } from "@/components/ui/skeleton"

import { useCreateWhiteboard } from "@/modules/whiteboard/api/use-whiteboards"
import { useCreateNote } from "@/modules/notes/api/use-notes"

import { useWorkspaceBoards, useWorkspaceNotes } from "../api/use-workspaces"
import { WorkspaceLibraryTable } from "./workspace-library-table"
import { WorkspaceHeader } from "./workspace-header"

type WorkspaceViewProps = {
  workspace?: Workspace
}

const tabs = [
  { id: "whiteboards", label: "Whiteboards" },
  { id: "notes", label: "Notes" },
]

export function WorkspaceView({ workspace }: WorkspaceViewProps) {
  const router = useRouter()
  const [activeTab, setActiveTab] = useState<(typeof tabs)[number]["id"]>("whiteboards")
  const createWhiteboardMutation = useCreateWhiteboard()
  const createNoteMutation = useCreateNote()

  const whiteboardsQuery = useWorkspaceBoards(workspace?.id ?? "", activeTab === "whiteboards")
  const notesQuery = useWorkspaceNotes(workspace?.id ?? "", activeTab === "notes")

  const handleCreateWhiteboard = async () => {
    if (!workspace || activeTab !== "whiteboards" || createWhiteboardMutation.isPending) {
      return
    }

    const createdBoard = await createWhiteboardMutation.mutateAsync({
      workspaceId: workspace.id,
    })

    if (createdBoard.title) {
      router.push(`/whiteboard/${encodeURIComponent(createdBoard.title)}`)
    }
  }

  const handleCreateNote = async () => {
    if (!workspace || activeTab !== "notes" || createNoteMutation.isPending) {
      return
    }

    const createdNote = await createNoteMutation.mutateAsync({
      workspaceId: workspace.id,
    })

    if (createdNote.title) {
      router.push(`/notes/${encodeURIComponent(createdNote.title)}`)
    }
  }

  const libraryView = useMemo(() => {
    if (activeTab === "notes") {
      return {
        title: "Workspace notes",
        subtitle: "Recent notes",
        emptyState: "No notes have been created in this workspace yet.",
        icon: "notes" as const,
        items: notesQuery.data ?? [],
        isLoading: notesQuery.isLoading,
      }
    }

    return {
      title: "Workspace whiteboards",
      subtitle: "Recent whiteboards",
      emptyState: "No whiteboards have been created in this workspace yet.",
      icon: "whiteboards" as const,
      items: whiteboardsQuery.data ?? [],
      isLoading: whiteboardsQuery.isLoading,
    }
  }, [
    activeTab,
    notesQuery.data,
    notesQuery.isLoading,
    whiteboardsQuery.data,
    whiteboardsQuery.isLoading,
  ])

  if (!workspace) {
    return (
      <div className="flex min-h-full flex-1 items-center justify-center p-6">
        <div className="w-full max-w-7xl space-y-4">
          <Skeleton className="h-55 w-full rounded-4xl" />
          <Skeleton className="h-24 w-full rounded-2xl" />
          <Skeleton className="h-105 w-full rounded-3xl" />
        </div>
      </div>
    )
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto bg-background">
      <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-4 sm:px-6 lg:px-8 lg:py-6">
        <WorkspaceHeader workspace={workspace} />

        <section className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
          <div className="space-y-1">
            <p className="text-xs font-medium uppercase tracking-[0.26em] text-muted-foreground">
              Library
            </p>
            <h2 className="text-3xl font-semibold tracking-tight text-foreground sm:text-4xl">
              {workspace.name || "Workspace"}
            </h2>
            <p className="max-w-2xl text-sm text-muted-foreground sm:text-base">
              Switch between the real workspace notes and whiteboards stored in this workspace.
            </p>
          </div>

          <div className="flex items-center gap-3">
            <Button
              className="rounded-full bg-card-tint-lavender"
              disabled={
                (activeTab === "whiteboards" && createWhiteboardMutation.isPending) ||
                (activeTab === "notes" && createNoteMutation.isPending)
              }
              onClick={() =>
                activeTab === "notes" ? void handleCreateNote() : void handleCreateWhiteboard()
              }
              size="sm"
              variant="outline"
            >
              <Plus className="mr-2 size-4" />
              {activeTab === "notes"
                ? createNoteMutation.isPending
                  ? "Creating…"
                  : "New note"
                : createWhiteboardMutation.isPending
                  ? "Creating…"
                  : "New whiteboard"}
            </Button>
            <Button className="rounded-full px-5 bg-card-tint-mint" size="sm" variant="outline">
              <Plus className="mr-2 size-4" />
              New page
            </Button>
          </div>
        </section>

        <div className="flex flex-wrap items-center justify-between gap-4 border-b border-border/60 pb-4">
          <AnimatedTabs
            className="bg-card-tint-peach/50"
            activeTab={activeTab}
            layoutId="workspace-library-tabs"
            onChange={setActiveTab}
            tabs={tabs}
            variant="pill"
          />
          <p className="text-sm text-muted-foreground">
            {activeTab === "notes"
              ? "Browse notes created in this workspace."
              : "Browse whiteboards created in this workspace."}
          </p>
        </div>

        <WorkspaceLibraryTable
          emptyState={libraryView.emptyState}
          icon={libraryView.icon}
          isLoading={libraryView.isLoading}
          items={libraryView.items}
          subtitle={libraryView.subtitle}
          title={libraryView.title}
        />
      </div>
    </div>
  )
}
