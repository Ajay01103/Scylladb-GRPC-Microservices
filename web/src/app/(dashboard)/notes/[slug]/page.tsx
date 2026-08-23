import { Suspense } from "react"
import { dehydrate, HydrationBoundary } from "@tanstack/react-query"
import { Loader2 } from "lucide-react"

import { getQueryClient } from "@/lib/get-query-client"
import { prefetchNoteBySlug } from "@/modules/notes/api/notes-server-queries"
import { NoteView } from "@/modules/notes/components/note-view"

type NotePageProps = {
  params: Promise<{ slug: string }>
}

export default async function NotePage({ params }: NotePageProps) {
  const { slug } = await params
  const queryClient = getQueryClient()
  await prefetchNoteBySlug(queryClient, slug)

  return (
    <HydrationBoundary state={dehydrate(queryClient)}>
      <Suspense
        fallback={
          <div className="flex min-h-0 flex-1 items-center justify-center text-sm text-muted-foreground">
            <div className="flex items-center gap-2">
              <Loader2 className="size-4 animate-spin" />
              Loading note…
            </div>
          </div>
        }
      >
        <NoteView slug={slug} />
      </Suspense>
    </HydrationBoundary>
  )
}
