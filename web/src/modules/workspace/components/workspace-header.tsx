import { Sparkles } from "lucide-react"

import { Button } from "@/components/ui/button"
import type { PlainWorkspace } from "@/modules/workspace/api/workspace-queries"

type WorkspaceHeaderProps = {
  workspace: Pick<PlainWorkspace, "id" | "name">
}

export function WorkspaceHeader({ workspace }: WorkspaceHeaderProps) {
  return (
    <section className="relative overflow-hidden rounded-[32px] border border-border/60 bg-card shadow-sm">
      <div
        className="absolute inset-0"
        style={{
          backgroundImage:
            "radial-gradient(circle at 18% 18%, rgba(255,255,255,0.9), transparent 24%), radial-gradient(circle at 82% 0%, rgba(255,255,255,0.65), transparent 22%), linear-gradient(135deg, #bfd3e9 0%, #dfe7f0 38%, #f6efe3 100%)",
        }}
      />
      <div className="absolute inset-0 opacity-20 [background-image:linear-gradient(90deg,rgba(255,255,255,0.45)_1px,transparent_1px),linear-gradient(rgba(255,255,255,0.45)_1px,transparent_1px)] [background-size:28px_28px]" />

      <div className="relative flex min-h-[180px] flex-col justify-between gap-6 p-6 sm:min-h-[220px] sm:p-8">
        <div className="flex items-start justify-between gap-4">
          <div className="max-w-2xl space-y-4 text-slate-900">
            {/* <p className="text-xs font-medium uppercase tracking-[0.3em] text-slate-700/80">
              Workspace theme space
            </p> */}
            <h1 className="text-4xl font-semibold tracking-tight sm:text-5xl">
              {workspace.name || workspace.id}
            </h1>
            {/* <p className="max-w-xl text-sm text-slate-800/75 sm:text-base">
              Cover image upload will plug in here later. For now this is a themed
              placeholder for the workspace header.
            </p> */}
          </div>

          <Button
            className="hidden rounded-full bg-white/90 text-slate-900 shadow-sm hover:bg-white sm:inline-flex"
            size="sm"
            variant="secondary"
          >
            Add cover
          </Button>
        </div>

        <div className="flex items-center justify-between gap-3 rounded-2xl border border-white/50 bg-white/55 px-4 py-3 backdrop-blur-md">
          <div>
            {/* <p className="text-xs uppercase tracking-[0.22em] text-slate-700/70">
              Workspace ID
            </p> */}
            {/* <p className="mt-1 text-sm font-medium text-slate-900">{workspace.id}</p>*/}
            <p className="max-w-xl text-sm text-slate-800/75 sm:text-base">
              Cover image upload will plug in here later. For now this is a themed placeholder for
              the workspace header.
            </p>
          </div>

          <div className="flex items-center gap-2 text-slate-700">
            <Sparkles className="size-4" />
            <span className="text-sm font-medium">Theme only for now</span>
          </div>
        </div>
      </div>
    </section>
  )
}
