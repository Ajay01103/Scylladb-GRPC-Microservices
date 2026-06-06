import { WhiteboardView } from "@/modules/whiteboard/components/whiteboard-view"

type WhiteboardPageProps = {
  params: Promise<{
    slug?: string
  }>
}

export default async function WhiteboardPage({ params }: WhiteboardPageProps) {
  const { slug } = await params

  return <WhiteboardView slug={slug} />
}
