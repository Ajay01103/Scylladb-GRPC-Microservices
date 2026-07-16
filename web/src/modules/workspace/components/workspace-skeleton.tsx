import { Skeleton } from "@/components/ui/skeleton"

export function WorkspaceViewSkeleton() {
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto bg-background">
      <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-4 sm:px-6 lg:px-8 lg:py-6">
        {/* WorkspaceHeader */}
        <section className="relative overflow-hidden rounded-[32px] border border-border/60 bg-card shadow-sm">
          <div className="absolute inset-0 bg-gradient-to-br from-slate-100 via-slate-50 to-amber-50/40" />
          <div className="absolute inset-0 opacity-20 [background-image:linear-gradient(90deg,rgba(255,255,255,0.45)_1px,transparent_1px),linear-gradient(rgba(255,255,255,0.45)_1px,transparent_1px)] [background-size:28px_28px]" />
          <div className="relative flex min-h-[180px] flex-col justify-between gap-6 p-6 sm:min-h-[220px] sm:p-8">
            <div className="flex items-start justify-between gap-4">
              <div className="max-w-2xl space-y-4">
                <Skeleton className="h-11 w-64 rounded-lg bg-slate-200/60" />
              </div>
              <Skeleton className="hidden h-8 w-24 rounded-full bg-white/60 sm:block" />
            </div>
            <div className="flex items-center justify-between gap-3 rounded-2xl border border-white/50 bg-white/55 px-4 py-3 backdrop-blur-md">
              <Skeleton className="h-4 w-80 rounded bg-slate-200/50" />
              <div className="flex items-center gap-2">
                <Skeleton className="size-4 rounded bg-slate-200/50" />
                <Skeleton className="h-4 w-24 rounded bg-slate-200/50" />
              </div>
            </div>
          </div>
        </section>

        {/* Library section */}
        <section className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
          <div className="space-y-2">
            <Skeleton className="h-3 w-16 rounded" />
            <Skeleton className="h-9 w-48 rounded-lg" />
            <Skeleton className="h-4 w-80 rounded" />
          </div>
          <div className="flex items-center gap-3">
            <Skeleton className="h-8 w-32 rounded-full" />
            <Skeleton className="h-8 w-28 rounded-full" />
          </div>
        </section>

        {/* Tab bar */}
        <div className="flex flex-wrap items-center justify-between gap-4 border-b border-border/60 pb-4">
          <div className="flex gap-1 rounded-full bg-card-tint-peach/50 p-1">
            <Skeleton className="h-8 w-28 rounded-full" />
            <Skeleton className="h-8 w-20 rounded-full" />
          </div>
          <Skeleton className="h-4 w-52 rounded" />
        </div>

        {/* WorkspaceLibraryTable */}
        <section className="rounded-3xl border border-border/70 bg-card/90 shadow-sm backdrop-blur">
          <div className="border-b border-border/60 px-6 py-5">
            <div className="space-y-2">
              <Skeleton className="h-3 w-20 rounded" />
              <Skeleton className="h-5 w-40 rounded" />
              <Skeleton className="h-4 w-72 rounded" />
            </div>
          </div>
          <div className="flex items-center gap-3 border-b border-border/60 px-6 py-4">
            <Skeleton className="h-10 w-full max-w-xs rounded-xl" />
            <Skeleton className="h-8 w-24 rounded-full" />
          </div>
          <div className="space-y-0">
            {Array.from({ length: 5 }).map((_, i) => (
              <div
                className="flex items-center gap-4 border-b border-border/60 px-6 py-4"
                key={i}
              >
                <Skeleton className="size-9 shrink-0 rounded-full" />
                <div className="flex-1 space-y-2">
                  <Skeleton className="h-4 w-40 rounded" />
                  <Skeleton className="h-3 w-28 rounded" />
                </div>
                <Skeleton className="h-6 w-16 rounded-full" />
                <Skeleton className="h-4 w-20 rounded" />
              </div>
            ))}
          </div>
          <div className="flex items-center justify-between border-t border-border/60 px-6 py-4">
            <Skeleton className="h-4 w-32 rounded" />
            <div className="flex gap-2">
              <Skeleton className="h-8 w-24 rounded-full" />
              <Skeleton className="h-8 w-20 rounded-full" />
            </div>
          </div>
        </section>
      </div>
    </div>
  )
}
