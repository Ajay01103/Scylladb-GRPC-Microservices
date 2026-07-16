import { Skeleton } from "@/components/ui/skeleton"

function StatCardSkeleton() {
  return (
    <div className="rounded-3xl border border-border/60 bg-card p-5 shadow-sm">
      <div className="flex items-center justify-between gap-3">
        <div className="space-y-3">
          <Skeleton className="h-3 w-24 rounded" />
          <Skeleton className="h-8 w-12 rounded" />
        </div>
        <Skeleton className="size-11 shrink-0 rounded-2xl" />
      </div>
    </div>
  )
}

function TableRowSkeleton() {
  return (
    <div className="flex items-center gap-4 border-b border-border/60 px-4 py-4 sm:px-6">
      <Skeleton className="size-9 shrink-0 rounded-full" />
      <div className="min-w-0 flex-1 space-y-2">
        <Skeleton className="h-4 w-36 rounded" />
      </div>
      <Skeleton className="h-4 w-44 shrink-0 rounded" />
      <Skeleton className="h-6 w-16 shrink-0 rounded-full" />
      <Skeleton className="h-6 w-14 shrink-0 rounded-full" />
      <Skeleton className="h-4 w-24 shrink-0 rounded" />
      <Skeleton className="h-4 w-20 shrink-0 rounded" />
      <Skeleton className="size-8 shrink-0 rounded-full" />
    </div>
  )
}

export function MembersSkeleton() {
  return (
    <div className="min-h-full flex-1 bg-background">
      <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-4 sm:px-6 lg:px-8 lg:py-6">
        {/* Header section */}
        <section className="flex flex-col gap-4 rounded-[32px] border border-border/60 bg-card px-6 py-6 shadow-sm sm:px-8 lg:flex-row lg:items-end lg:justify-between">
          <div className="space-y-3">
            <Skeleton className="h-3 w-36 rounded" />
            <Skeleton className="h-9 w-56 rounded-lg" />
            <Skeleton className="h-4 w-80 rounded" />
          </div>
          <Skeleton className="h-8 w-32 shrink-0 rounded-full" />
        </section>

        {/* Stats grid */}
        <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <StatCardSkeleton />
          <StatCardSkeleton />
          <StatCardSkeleton />
          <StatCardSkeleton />
        </section>

        {/* Members table */}
        <section className="rounded-3xl border border-border/70 bg-card/90 shadow-sm backdrop-blur">
          <div className="border-b border-border/60 px-4 py-4 sm:px-6">
            <div className="space-y-2">
              <Skeleton className="h-3 w-28 rounded" />
              <Skeleton className="h-5 w-36 rounded" />
              <Skeleton className="h-4 w-80 rounded" />
            </div>
          </div>

          {/* Filter bar */}
          <div className="flex flex-col gap-4 border-b border-border/60 px-4 py-4 sm:px-6 lg:flex-row lg:items-center lg:justify-between">
            <div className="grid flex-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
              <div className="space-y-2">
                <Skeleton className="h-3 w-14 rounded" />
                <Skeleton className="h-10 w-full rounded-xl" />
              </div>
              <div className="space-y-2">
                <Skeleton className="h-3 w-10 rounded" />
                <Skeleton className="h-10 w-full rounded-xl" />
              </div>
              <div className="space-y-2">
                <Skeleton className="h-3 w-12 rounded" />
                <Skeleton className="h-10 w-full rounded-xl" />
              </div>
              <div className="space-y-2">
                <Skeleton className="h-3 w-14 rounded" />
                <div className="flex h-10 items-center">
                  <Skeleton className="h-8 w-28 rounded-xl" />
                </div>
              </div>
            </div>
          </div>

          {/* Count bar */}
          <div className="flex items-center justify-between px-4 py-3 sm:px-6">
            <Skeleton className="h-4 w-36 rounded" />
            <Skeleton className="hidden h-4 w-64 rounded sm:block" />
          </div>

          {/* Table header */}
          <div className="flex items-center gap-4 border-b border-border/60 bg-muted/20 px-4 py-3 sm:px-6">
            <Skeleton className="h-4 w-20 rounded" />
            <Skeleton className="h-4 w-14 rounded" />
            <Skeleton className="h-4 w-10 rounded" />
            <Skeleton className="h-4 w-14 rounded" />
            <Skeleton className="h-4 w-14 rounded" />
            <Skeleton className="h-4 w-18 rounded" />
            <Skeleton className="h-4 w-16 rounded" />
          </div>

          {/* Table rows */}
          {Array.from({ length: 6 }).map((_, i) => (
            <TableRowSkeleton key={i} />
          ))}

          {/* Pagination */}
          <div className="flex items-center justify-between border-t border-border/60 px-4 py-4 sm:px-6">
            <Skeleton className="h-4 w-32 rounded" />
            <div className="flex gap-2">
              <Skeleton className="h-8 w-24 rounded-full" />
              <Skeleton className="h-8 w-18 rounded-full" />
            </div>
          </div>
        </section>
      </div>
    </div>
  )
}
