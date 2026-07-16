import Link from "next/link"

import { FileText, LayoutGrid, Lock, Search, SlidersHorizontal } from "lucide-react"

import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

import type { WorkspaceLibraryItem } from "../api/use-workspaces"

type WorkspaceLibraryTableProps = {
  title: string
  subtitle: string
  emptyState: string
  icon: "notes" | "whiteboards"
  items: WorkspaceLibraryItem[]
}

const iconByKind = {
  notes: FileText,
  whiteboards: LayoutGrid,
}

export function WorkspaceLibraryTable({
  title,
  subtitle,
  emptyState,
  icon,
  items,
}: WorkspaceLibraryTableProps) {
  const ItemIcon = iconByKind[icon]
  const isWhiteboards = icon === "whiteboards"

  return (
    <section className="rounded-3xl border border-border/70 bg-card/90 shadow-sm backdrop-blur">
      <div className="flex items-center justify-between gap-4 border-b border-border/60 px-4 py-4 sm:px-6">
        <div>
          <p className="text-xs uppercase tracking-[0.22em] text-muted-foreground">{title}</p>
          <h3 className="mt-1 text-base font-semibold text-foreground">{subtitle}</h3>
        </div>

        <div className="flex items-center gap-1 text-muted-foreground">
          <Button
            aria-label="Search items"
            className="size-9 rounded-full"
            size="icon"
            variant="ghost"
          >
            <Search className="size-4" />
          </Button>
          <Button
            aria-label="Filter items"
            className="size-9 rounded-full"
            size="icon"
            variant="ghost"
          >
            <SlidersHorizontal className="size-4" />
          </Button>
        </div>
      </div>

      <Table>
        <TableHeader>
          <TableRow className="border-border/60 hover:bg-transparent">
            <TableHead className="w-[46%] px-6 py-3 text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
              Item title
            </TableHead>
            <TableHead className="w-[22%] px-6 py-3 text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
              Created by
            </TableHead>
            <TableHead className="w-[16%] px-6 py-3 text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
              Source
            </TableHead>
            <TableHead className="w-[16%] px-6 py-3 text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
              Last edited time
            </TableHead>
          </TableRow>
        </TableHeader>

        <TableBody>
          {items.length > 0 ? (
            items.map((item) => {
              return (
                <TableRow className="border-border/60 hover:bg-muted/30" key={item.id}>
                  <TableCell className="px-6 py-4">
                    {isWhiteboards ? (
                      <Link
                        className="flex items-center gap-3"
                        href={`/whiteboard/${encodeURIComponent(item.title)}`}
                        prefetch
                      >
                        <div className="flex size-8 items-center justify-center rounded-lg border border-border/60 bg-muted text-muted-foreground">
                          <ItemIcon className="size-4" />
                        </div>
                        <span className="text-sm font-medium text-foreground">{item.title}</span>
                      </Link>
                    ) : (
                      <Link
                        className="flex items-center gap-3"
                        href={`/notes/${encodeURIComponent(item.title)}`}
                        prefetch
                      >
                        <div className="flex size-8 items-center justify-center rounded-lg border border-border/60 bg-muted text-muted-foreground">
                          <ItemIcon className="size-4" />
                        </div>
                        <span className="text-sm font-medium text-foreground">{item.title}</span>
                      </Link>
                    )}
                  </TableCell>

                  <TableCell className="px-6 py-4">
                    <div className="flex items-center gap-3">
                      <Avatar size="sm">
                        <AvatarFallback className="bg-slate-900 text-[11px] font-medium text-white">
                          {item.createdBy.slice(0, 1).toUpperCase()}
                        </AvatarFallback>
                      </Avatar>
                      <span className="text-sm text-muted-foreground">{item.createdBy}</span>
                    </div>
                  </TableCell>

                  <TableCell className="px-6 py-4 text-sm text-muted-foreground">
                    <span className="inline-flex items-center gap-2 rounded-full border border-border/60 px-2.5 py-1 text-xs font-medium text-foreground">
                      <Lock className="size-3.5" />
                      {item.visibility}
                    </span>
                  </TableCell>

                  <TableCell className="px-6 py-4 text-sm text-muted-foreground">
                    {item.updatedAt}
                  </TableCell>
                </TableRow>
              )
            })
          ) : (
            <TableRow className="border-border/60 hover:bg-transparent">
              <TableCell
                className="px-6 py-12 text-center text-sm text-muted-foreground"
                colSpan={4}
              >
                {emptyState}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </section>
  )
}
