import { Loader2 } from "lucide-react"

export default function Loading() {
  return (
    <div className="flex min-h-0 flex-1 items-center justify-center bg-background text-sm text-muted-foreground">
      <div className="flex items-center gap-2">
        <Loader2 className="size-4 animate-spin" />
        Loading note...
      </div>
    </div>
  )
}
