import { Suspense } from "react"
import { dehydrate, HydrationBoundary } from "@tanstack/react-query"
import { Loader2 } from "lucide-react"

import { getQueryClient } from "@/lib/get-query-client"
import { prefetchWhiteboardBySlug } from "@/modules/whiteboard/api/whiteboard-server-queries"
import { WhiteboardView } from "@/modules/whiteboard/components/whiteboard-view"

type WhiteboardPageProps = {
  params: Promise<{
    slug: string
  }>
}

export default async function WhiteboardPage({ params }: WhiteboardPageProps) {
  const { slug } = await params
  const queryClient = getQueryClient()
  await prefetchWhiteboardBySlug(queryClient, slug)

  return (
    <HydrationBoundary state={dehydrate(queryClient)}>
      <Suspense
        fallback={
          <div className="flex min-h-svh items-center justify-center bg-background text-sm text-muted-foreground">
            <div className="flex items-center gap-2">
              <Loader2 className="size-4 animate-spin" />
              Loading whiteboard…
            </div>
          </div>
        }
      >
        <WhiteboardView slug={slug} />
      </Suspense>
    </HydrationBoundary>
  )
}
