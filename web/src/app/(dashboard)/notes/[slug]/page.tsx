import { NoteView } from "@/modules/notes/components/note-view"

type NotePageProps = {
  params: Promise<{ slug: string }>
}

export default async function NotePage({ params }: NotePageProps) {
  const { slug } = await params
  return <NoteView slug={slug} />
}
