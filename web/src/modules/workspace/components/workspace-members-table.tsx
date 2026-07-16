"use client"

import { useMemo, useState } from "react"

import {
  ArrowUpDown,
  ChevronLeft,
  ChevronRight,
  Clock3,
  Mail,
  MoreHorizontal,
  Search,
} from "lucide-react"

import type { ColumnDef, SortingState } from "@tanstack/react-table"
import {
  flexRender,
  getCoreRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
} from "@tanstack/react-table"

import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { WorkspaceRole, type WorkspaceMember } from "@/gen/pb/workspace/workspace_pb"

type WorkspaceMemberRow = {
  name: string
  email: string
  userId: string
  role: string
  invitedBy: string
  invitedByLabel: string
  joinedAt: string
  status: "Active" | "Invited"
}

function deriveDisplayFromMember(member: WorkspaceMember) {
  const backendName = member.userName.trim()
  const backendEmail = member.userEmail.trim()

  return {
    name: backendName || compactIdentity(member.userId),
    email: backendEmail || "Not available",
  }
}

function compactIdentity(value: string) {
  const normalized = value.trim()

  if (!normalized) {
    return "-"
  }

  if (normalized.length <= 18) {
    return normalized
  }

  return `${normalized.slice(0, 8)}...${normalized.slice(-6)}`
}

function toRoleLabel(role: WorkspaceRole | string | number | null | undefined) {
  if (typeof role === "string") {
    const normalized = role.trim()
    if (normalized === "WORKSPACE_ROLE_OWNER") return "Owner"
    if (normalized === "WORKSPACE_ROLE_ADMIN") return "Admin"
    if (normalized === "WORKSPACE_ROLE_EDITOR") return "Editor"
    if (normalized === "WORKSPACE_ROLE_MEMBER") return "Member"
    if (normalized === "WORKSPACE_ROLE_GUEST") return "Guest"
  }

  switch (role) {
    case WorkspaceRole.OWNER:
      return "Owner"
    case WorkspaceRole.ADMIN:
      return "Admin"
    case WorkspaceRole.EDITOR:
      return "Editor"
    case WorkspaceRole.MEMBER:
      return "Member"
    case WorkspaceRole.GUEST:
      return "Guest"
    default:
      return "Unknown"
  }
}

function formatJoinedAt(value: unknown) {
  if (value == null) {
    return "Pending invitation"
  }

  if (typeof value === "string") {
    const parsed = Date.parse(value)

    if (!Number.isFinite(parsed)) {
      return "Unknown"
    }

    return new Intl.DateTimeFormat("en-US", {
      month: "short",
      day: "numeric",
      year: "numeric",
    }).format(new Date(parsed))
  }

  if (typeof value === "object") {
    const secondsValue = (value as { seconds?: bigint | number | string }).seconds
    const nanosValue = (value as { nanos?: number | string }).nanos

    if (secondsValue == null) {
      return "Pending invitation"
    }

    const secondsAsNumber =
      typeof secondsValue === "bigint"
        ? Number(secondsValue)
        : typeof secondsValue === "string"
          ? Number(secondsValue)
          : secondsValue
    const nanosAsNumber = typeof nanosValue === "string" ? Number(nanosValue) : (nanosValue ?? 0)

    const millis = secondsAsNumber * 1000 + Math.floor(nanosAsNumber / 1_000_000)

    if (!Number.isFinite(millis)) {
      return "Unknown"
    }

    return new Intl.DateTimeFormat("en-US", {
      month: "short",
      day: "numeric",
      year: "numeric",
    }).format(new Date(millis))
  }

  return "Unknown"
}

function toRow(member: WorkspaceMember): WorkspaceMemberRow {
  const role = toRoleLabel(member.role as WorkspaceRole | string | number)
  const invitedBy = member.invitedBy.trim()
  const { name, email } = deriveDisplayFromMember(member)
  const joinedAt = formatJoinedAt(member.joinedAt as unknown)
  const invitedByName = (member.invitedByName ?? "").trim()
  const invitedByEmail = (member.invitedByEmail ?? "").trim()
  const invitedByLabel = invitedByName || (invitedBy ? compactIdentity(invitedBy) : "-")
  const invitedByTooltip = invitedBy
    ? Array.from(new Set([invitedByName, invitedByEmail, invitedBy].filter(Boolean))).join(" • ")
    : "-"

  return {
    name,
    email,
    userId: member.userId,
    role,
    invitedBy: invitedByTooltip,
    invitedByLabel,
    joinedAt,
    status: joinedAt === "Pending invitation" ? "Invited" : "Active",
  }
}

const roleTone: Record<string, string> = {
  Owner: "border-amber-200 bg-amber-50 text-amber-900",
  Admin: "border-sky-200 bg-sky-50 text-sky-900",
  Editor: "border-indigo-200 bg-indigo-50 text-indigo-900",
  Member: "border-emerald-200 bg-emerald-50 text-emerald-900",
  Guest: "border-slate-200 bg-slate-50 text-slate-900",
}

const roleOptions = ["all", "Owner", "Admin", "Editor", "Member", "Guest"] as const
const statusOptions = ["all", "Active", "Invited"] as const

function getInitials(name: string) {
  return name
    .split(" ")
    .filter(Boolean)
    .map((part) => part[0])
    .slice(0, 2)
    .join("")
    .toUpperCase()
}

function SortableHeader({ title, onClick }: { title: string; onClick: () => void }) {
  return (
    <Button
      className="-ml-3 h-8 gap-2 rounded-full px-3 text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground hover:text-foreground"
      onClick={onClick}
      size="sm"
      variant="ghost"
    >
      <span>{title}</span>
      <ArrowUpDown className="size-3.5" />
    </Button>
  )
}

type WorkspaceMembersTableProps = {
  members: WorkspaceMember[]
}

export function WorkspaceMembersTable({ members }: WorkspaceMembersTableProps) {
  const [searchQuery, setSearchQuery] = useState("")
  const [roleFilter, setRoleFilter] = useState<(typeof roleOptions)[number]>("all")
  const [statusFilter, setStatusFilter] = useState<(typeof statusOptions)[number]>("all")
  const [sorting, setSorting] = useState<SortingState>([{ id: "name", desc: false }])

  const rows = useMemo(() => members.map(toRow), [members])

  const filteredMembers = useMemo(() => {
    const normalizedSearch = searchQuery.trim().toLowerCase()

    return rows.filter((member) => {
      const searchableText = [
        member.name,
        member.email,
        member.userId,
        member.role,
        member.invitedBy,
        member.invitedByLabel,
        member.joinedAt,
        member.status,
      ]
        .join(" ")
        .toLowerCase()

      const matchesSearch = !normalizedSearch || searchableText.includes(normalizedSearch)
      const matchesRole = roleFilter === "all" || member.role === roleFilter
      const matchesStatus = statusFilter === "all" || member.status === statusFilter

      return matchesSearch && matchesRole && matchesStatus
    })
  }, [roleFilter, rows, searchQuery, statusFilter])

  const columns = useMemo<ColumnDef<WorkspaceMemberRow>[]>(
    () => [
      {
        accessorKey: "name",
        header: ({ column }) => (
          <SortableHeader
            onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
            title="Member"
          />
        ),
        cell: ({ row }) => {
          const member = row.original

          return (
            <div className="flex min-w-0 items-center gap-3">
              <Avatar size="sm">
                <AvatarFallback className="bg-slate-900 text-[11px] font-medium text-white">
                  {getInitials(member.name)}
                </AvatarFallback>
              </Avatar>
              <div className="min-w-0 space-y-1">
                <p
                  className="truncate text-sm font-semibold text-foreground"
                  title={`${member.name} (${member.userId})`}
                >
                  {member.name}
                </p>
              </div>
            </div>
          )
        },
      },
      {
        accessorKey: "email",
        header: ({ column }) => (
          <SortableHeader
            onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
            title="Email"
          />
        ),
        cell: ({ getValue }) => {
          const email = String(getValue())
          const isAvailable = email !== "Not available"

          return (
            <div className="flex min-w-0 items-center gap-1.5">
              <Mail className="size-3.5 shrink-0 text-muted-foreground" />
              <span
                className={`block truncate text-sm ${isAvailable ? "text-foreground" : "text-muted-foreground"}`}
                title={email}
              >
                {email}
              </span>
            </div>
          )
        },
      },
      {
        accessorKey: "role",
        header: ({ column }) => (
          <SortableHeader
            onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
            title="Role"
          />
        ),
        cell: ({ getValue }) => {
          const role = String(getValue())

          return (
            <span
              className={`inline-flex items-center rounded-full border px-2.5 py-1 text-xs font-medium ${roleTone[role] ?? "border-border/60 bg-muted text-foreground"}`}
            >
              {role}
            </span>
          )
        },
      },
      {
        accessorKey: "status",
        header: ({ column }) => (
          <SortableHeader
            onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
            title="Status"
          />
        ),
        cell: ({ getValue }) => {
          const status = String(getValue())

          return (
            <span
              className={`inline-flex items-center rounded-full border px-2.5 py-1 text-xs font-medium ${status === "Active" ? "border-emerald-200 bg-emerald-50 text-emerald-900" : "border-amber-200 bg-amber-50 text-amber-900"}`}
            >
              {status}
            </span>
          )
        },
      },
      {
        accessorKey: "joinedAt",
        header: ({ column }) => (
          <SortableHeader
            onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
            title="Joined"
          />
        ),
      },
      {
        accessorKey: "invitedByLabel",
        header: ({ column }) => (
          <SortableHeader
            onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
            title="Invited by"
          />
        ),
        cell: ({ row }) => (
          <span
            className="block max-w-44 truncate font-mono text-xs text-muted-foreground"
            title={row.original.invitedBy}
          >
            {row.original.invitedByLabel}
          </span>
        ),
      },
      {
        id: "actions",
        header: () => <span className="sr-only">Actions</span>,
        cell: ({ row }) => (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                aria-label={`Open actions for ${row.original.name}`}
                className="size-8 rounded-full"
                size="icon"
                variant="ghost"
              >
                <MoreHorizontal className="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-52">
              <DropdownMenuItem>View profile</DropdownMenuItem>
              <DropdownMenuItem>Copy user ID</DropdownMenuItem>
              <DropdownMenuItem>Message member</DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem className="text-destructive focus:text-destructive">
                Remove access
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        ),
      },
    ],
    [],
  )

  const table = useReactTable({
    data: filteredMembers,
    columns,
    state: {
      sorting,
    },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  })

  const visibleCount = table.getRowModel().rows.length
  const totalCount = rows.length

  return (
    <section className="rounded-3xl border border-border/70 bg-card/90 shadow-sm backdrop-blur">
      <div className="border-b border-border/60 px-4 py-4 sm:px-6">
        <div className="space-y-1">
          <p className="text-xs uppercase tracking-[0.22em] text-muted-foreground">
            Workspace access
          </p>
          <h3 className="text-base font-semibold text-foreground">Members directory</h3>
          <p className="max-w-2xl text-sm text-muted-foreground">
            Search, filter, sort, and page through workspace members loaded from the workspace
            service.
          </p>
        </div>
      </div>

      <div className="flex flex-col gap-4 border-b border-border/60 px-4 py-4 sm:px-6 lg:flex-row lg:items-center lg:justify-between">
        <div className="grid flex-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <label className="space-y-2">
            <span className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
              Search
            </span>
            <div className="relative">
              <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                className="h-10 rounded-xl border-border/60 bg-background pl-9"
                onChange={(event) => setSearchQuery(event.target.value)}
                placeholder="Search member, email, role, or ID"
                value={searchQuery}
              />
            </div>
          </label>

          <label className="space-y-2">
            <span className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
              Role
            </span>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  className="h-10 w-full justify-between rounded-xl border-border/60 bg-background px-3 text-left"
                  variant="outline"
                >
                  <span>{roleFilter === "all" ? "All roles" : roleFilter}</span>
                  <ArrowUpDown className="size-4 text-muted-foreground" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start" className="w-48">
                <DropdownMenuRadioGroup
                  onValueChange={(value) => setRoleFilter(value as (typeof roleOptions)[number])}
                  value={roleFilter}
                >
                  {roleOptions.map((role) => (
                    <DropdownMenuRadioItem key={role} value={role}>
                      {role === "all" ? "All roles" : role}
                    </DropdownMenuRadioItem>
                  ))}
                </DropdownMenuRadioGroup>
              </DropdownMenuContent>
            </DropdownMenu>
          </label>

          <label className="space-y-2">
            <span className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
              Status
            </span>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  className="h-10 w-full justify-between rounded-xl border-border/60 bg-background px-3 text-left"
                  variant="outline"
                >
                  <span>{statusFilter === "all" ? "All statuses" : statusFilter}</span>
                  <Clock3 className="size-4 text-muted-foreground" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start" className="w-48">
                <DropdownMenuRadioGroup
                  onValueChange={(value) =>
                    setStatusFilter(value as (typeof statusOptions)[number])
                  }
                  value={statusFilter}
                >
                  {statusOptions.map((status) => (
                    <DropdownMenuRadioItem key={status} value={status}>
                      {status === "all" ? "All statuses" : status}
                    </DropdownMenuRadioItem>
                  ))}
                </DropdownMenuRadioGroup>
              </DropdownMenuContent>
            </DropdownMenu>
          </label>

          <div className="space-y-2">
            <span className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
              Actions
            </span>
            <div className="flex h-10 items-center gap-2">
              <Button
                className="h-10 rounded-xl px-3"
                onClick={() => {
                  setSearchQuery("")
                  setRoleFilter("all")
                  setStatusFilter("all")
                }}
                variant="outline"
              >
                Clear filters
              </Button>
            </div>
          </div>
        </div>
      </div>

      <div className="flex items-center justify-between gap-3 px-4 py-3 text-sm text-muted-foreground sm:px-6">
        <p>
          Showing <span className="font-medium text-foreground">{visibleCount}</span> of{" "}
          <span className="font-medium text-foreground">{totalCount}</span> members
        </p>
        <p className="hidden sm:block">
          Sort any column, filter by role or status, and page through the list.
        </p>
      </div>

      <div className="overflow-hidden border-t border-border/60">
        <Table className="min-w-[900px] table-fixed">
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow
                className="border-border/60 bg-muted/20 hover:bg-transparent"
                key={headerGroup.id}
              >
                {headerGroup.headers.map((header) => (
                  <TableHead className="px-4 py-3 align-middle" key={header.id}>
                    {header.isPlaceholder
                      ? null
                      : flexRender(header.column.columnDef.header, header.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>

          <TableBody>
            {table.getRowModel().rows.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow className="border-border/60 hover:bg-muted/30" key={row.id}>
                  {row.getVisibleCells().map((cell) => (
                    <TableCell
                      className={`px-4 py-4 align-top ${cell.column.id === "name" ? "max-w-[16rem] whitespace-normal" : ""} ${cell.column.id === "email" ? "max-w-[17rem]" : ""} ${cell.column.id === "invitedByLabel" ? "max-w-[12rem]" : ""}`}
                      key={cell.id}
                    >
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell
                  className="h-28 px-4 text-center text-sm text-muted-foreground"
                  colSpan={columns.length}
                >
                  No members match the current filters.
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      <div className="flex flex-col gap-4 border-t border-border/60 px-4 py-4 sm:flex-row sm:items-center sm:justify-between sm:px-6">
        <p className="text-sm text-muted-foreground">
          Page{" "}
          <span className="font-medium text-foreground">
            {table.getState().pagination.pageIndex + 1}
          </span>{" "}
          of <span className="font-medium text-foreground">{table.getPageCount()}</span>
        </p>

        <div className="flex items-center gap-2">
          <Button
            className="rounded-full"
            disabled={!table.getCanPreviousPage()}
            onClick={() => table.previousPage()}
            size="sm"
            variant="outline"
          >
            <ChevronLeft className="mr-2 size-4" />
            Previous
          </Button>
          <Button
            className="rounded-full"
            disabled={!table.getCanNextPage()}
            onClick={() => table.nextPage()}
            size="sm"
            variant="outline"
          >
            Next
            <ChevronRight className="ml-2 size-4" />
          </Button>
        </div>
      </div>
    </section>
  )
}
