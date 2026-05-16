"use client"

import AnimatedTabs from "@/components/ui/animated-tabs"
import {
  Combobox,
  ComboboxContent,
  ComboboxCreateNew,
  ComboboxEmpty,
  ComboboxGroup,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
  ComboboxTrigger,
} from "@/components/ui/combobox"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
  SidebarTrigger,
} from "@/components/ui/sidebar"
import { UserButton } from "@/components/user-button"
import {
  buildWorkspaceSlug,
  useCreateWorkspace,
  useMyWorkspaces,
} from "@/modules/workspace/api/use-workspaces"
import { useWorkspaceUrlState } from "@/modules/workspace/api/use-workspace-url-state"
import {
  type LucideIcon,
  Settings,
  Headphones,
} from "lucide-react"
import Image from "next/image"
import Link from "next/link"
import { useParams, usePathname, useRouter } from "next/navigation"
import { useEffect, useMemo, useState, type KeyboardEvent } from "react"

interface MenuItem {
  title: string
  url?: string
  icon: LucideIcon
  onClick?: () => void
}

interface NavSectionProps {
  label?: string
  items: MenuItem[]
  pathname: string
}

const NavSection = ({ label, items, pathname }: NavSectionProps) => {
  return (
    <SidebarGroup>
      {label && (
        <SidebarGroupLabel className="text-[13px] uppercase text-muted-foreground">
          {label}
        </SidebarGroupLabel>
      )}
      <SidebarGroupContent>
        <SidebarMenu>
          {items.map((item) => (
            <SidebarMenuItem key={item.title}>
              {item.url ? (
                <Link href={item.url}>
                  <SidebarMenuButton
                    isActive={
                      item.url === "/" ? pathname === "/" : pathname.startsWith(item.url)
                    }
                    tooltip={item.title}
                    className="h-9 px-3 py-2 text-[13px] tracking-tight font-medium border border-transparent data-[active=true]:border-border data-[active=true]:shadow-[0px_1px_1px_0px_rgba(44,54,53,0.03),inset_0px_0px_0px_2px_white]">
                    <item.icon />
                    <span>{item.title}</span>
                  </SidebarMenuButton>
                </Link>
              ) : (
                <SidebarMenuButton
                  isActive={false}
                  onClick={item.onClick}
                  tooltip={item.title}
                  className="h-9 px-3 py-2 text-[13px] tracking-tight font-medium border border-transparent data-[active=true]:border-border data-[active=true]:shadow-[0px_1px_1px_0px_rgba(44,54,53,0.03),inset_0px_0px_0px_2px_white]">
                  <item.icon />
                  <span>{item.title}</span>
                </SidebarMenuButton>
              )}
            </SidebarMenuItem>
          ))}
        </SidebarMenu>
      </SidebarGroupContent>
    </SidebarGroup>
  )
}

export const DashboardSidebar = () => {
  const pathname = usePathname()
  const router = useRouter()
  const params = useParams<{ id?: string | string[] }>()
  const routeWorkspaceId = Array.isArray(params?.id) ? params.id[0] : (params?.id ?? "")
  const [activeTab, setActiveTab] = useState("white-board")
  const [workspaceSearch, setWorkspaceSearch] = useState("")
  const [workspaceMenuOpen, setWorkspaceMenuOpen] = useState(false)
  const { setWorkspaceQuery, workspaceQuery } = useWorkspaceUrlState(routeWorkspaceId)

  const workspacesQuery = useMyWorkspaces()
  const createWorkspaceMutation = useCreateWorkspace()
  const workspaces = workspacesQuery.data ?? []
  const sortedWorkspaces = useMemo(
    () =>
      [...workspaces].sort((left, right) => {
        const leftCreatedAt = left.createdAt ? Number(left.createdAt.seconds ?? 0) : Number.MAX_SAFE_INTEGER
        const rightCreatedAt = right.createdAt ? Number(right.createdAt.seconds ?? 0) : Number.MAX_SAFE_INTEGER

        if (leftCreatedAt !== rightCreatedAt) {
          return leftCreatedAt - rightCreatedAt
        }

        return left.name.localeCompare(right.name)
      }),
    [workspaces],
  )
  const workspaceOptions = useMemo(
    () =>
      sortedWorkspaces.map((workspace) => ({
        value: workspace.id,
        label: workspace.name,
        keywords: [workspace.name, workspace.slug],
      })),
    [sortedWorkspaces],
  )
  const defaultWorkspaceId = workspaceOptions[0]?.value ?? ""
  const selectedWorkspaceId = routeWorkspaceId || workspaceQuery
  const effectiveSelectedWorkspaceId =
    workspaceOptions.some((workspace) => workspace.value === selectedWorkspaceId)
      ? selectedWorkspaceId
      : defaultWorkspaceId
  const selectedWorkspaceLabel =
    workspaceOptions.find((workspace) => workspace.value === effectiveSelectedWorkspaceId)
      ?.label ?? ""

  const filteredWorkspaces = useMemo(() => {
    const normalizedSearch = workspaceSearch.trim().toLowerCase()

    if (!normalizedSearch) {
      return workspaceOptions
    }

    return workspaceOptions.filter((workspace) =>
      workspace.label.toLowerCase().includes(normalizedSearch),
    )
  }, [workspaceOptions, workspaceSearch])

  const tabs = [
    { id: "white-board", label: "White Board" },
    { id: "notes", label: "Notes" },
  ]

  const mainMenuItems: MenuItem[] = [
    // main navigation intentionally minimal; dashboard-specific link removed
  ]

  const othersMenuItems: MenuItem[] = [
    {
      title: "Settings",
      icon: Settings,
      // onClick: () => clerk.openOrganizationProfile(),
    },
    {
      title: "Help and support",
      url: "mailto:business@codewithantonio.com",
      icon: Headphones,
    },
  ]

  const recentWhiteboards = [
    { id: "wb-1", title: "Sprint Planning", url: "/dashboard/whiteboard/1" },
    { id: "wb-2", title: "Product Vision", url: "/dashboard/whiteboard/2" },
    { id: "wb-3", title: "Design Notes", url: "/dashboard/whiteboard/3" },
    { id: "wb-4", title: "Roadmap", url: "/dashboard/whiteboard/4" },
    { id: "wb-5", title: "Retrospective", url: "/dashboard/whiteboard/5" },
    { id: "wb-6", title: "Ideas", url: "/dashboard/whiteboard/6" },
  ]

  const recentNotes = [
    { id: "n-1", title: "Meeting Notes", url: "/dashboard/notes/1" },
    { id: "n-2", title: "Research", url: "/dashboard/notes/2" },
    { id: "n-3", title: "Specs", url: "/dashboard/notes/3" },
    { id: "n-4", title: "User Feedback", url: "/dashboard/notes/4" },
    { id: "n-5", title: "Changelog", url: "/dashboard/notes/5" },
    { id: "n-6", title: "Backlog", url: "/dashboard/notes/6" },
  ]

  useEffect(() => {
    // Ensure the base /dashboard route shows white-board by default
    if (pathname === "/dashboard" || pathname === "/dashboard/") {
      setActiveTab("white-board")
    }
  }, [pathname])

  useEffect(() => {
    if (routeWorkspaceId && routeWorkspaceId !== workspaceQuery) {
      void setWorkspaceQuery(routeWorkspaceId)
    }
  }, [routeWorkspaceId, setWorkspaceQuery, workspaceQuery])

  useEffect(() => {
    if (!defaultWorkspaceId || routeWorkspaceId || workspaceQuery) {
      return
    }

    void setWorkspaceQuery(defaultWorkspaceId)
  }, [defaultWorkspaceId, routeWorkspaceId, setWorkspaceQuery, workspaceQuery])

  useEffect(() => {
    if (!workspaceMenuOpen) {
      setWorkspaceSearch("")
    }
  }, [workspaceMenuOpen])

  const handleWorkspaceChange = (workspaceId: string) => {
    if (!workspaceId) {
      return
    }

    void setWorkspaceQuery(workspaceId)
    setWorkspaceSearch("")
    setWorkspaceMenuOpen(false)
    router.push(`/workspace/${workspaceId}?workspace=${encodeURIComponent(workspaceId)}`)
  }

  const handleCreateWorkspace = async (workspaceName: string) => {
    const trimmedName = workspaceName.trim()

    if (!trimmedName) {
      return
    }

    const createdWorkspace = await createWorkspaceMutation.mutateAsync({
      name: trimmedName,
      slug: buildWorkspaceSlug(trimmedName),
      description: "",
      iconUrl: "",
      isPublic: false,
    })

    handleWorkspaceChange(createdWorkspace.id)
  }

  const handleWorkspaceSearchKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key !== "Enter") {
      return
    }

    const trimmedSearch = workspaceSearch.trim()

    if (!trimmedSearch || filteredWorkspaces.length > 0) {
      return
    }

    event.preventDefault()
    event.stopPropagation()
    void handleCreateWorkspace(trimmedSearch)
  }

  return (
    <>
      <Sidebar collapsible="icon">
        <SidebarHeader className="flex flex-col gap-3 pt-4">
          <div className="flex items-center gap-2 pl-1 group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:pl-0">
            <Image
              src="/logo.svg"
              alt="Resonance"
              width={24}
              height={24}
              className="rounded-sm"
            />
            <span className="group-data-[collapsible=icon]:hidden font-semibold text-lg tracking-tighter text-foreground">
              Resonance
            </span>
            <SidebarTrigger className="ml-auto group-data-[collapsible=icon]:hidden" />
          </div>
          <div className="group-data-[collapsible=icon]:hidden space-y-2 px-1">
            <SidebarGroupLabel className="px-1 text-[13px] uppercase text-muted-foreground">
              Workspace
            </SidebarGroupLabel>
            <Combobox
              data={workspaceOptions}
              type="workspace"
              onOpenChange={setWorkspaceMenuOpen}
              onValueChange={handleWorkspaceChange}
              open={workspaceMenuOpen}
              value={effectiveSelectedWorkspaceId}>
              <ComboboxTrigger className="h-10 w-full justify-between rounded-xl border-border/60 bg-background px-3 text-sm shadow-sm" />
              <ComboboxContent className="p-0">
                <ComboboxInput
                  onKeyDown={handleWorkspaceSearchKeyDown}
                  onValueChange={setWorkspaceSearch}
                  placeholder="Search workspaces..."
                />
                <ComboboxEmpty>
                  <ComboboxCreateNew onCreateNew={handleCreateWorkspace} />
                </ComboboxEmpty>
                <ComboboxList>
                  <ComboboxGroup>
                    {filteredWorkspaces.map((workspace) => (
                      <ComboboxItem
                        key={workspace.value}
                        keywords={workspace.keywords}
                        value={workspace.value}>
                        {workspace.label}
                      </ComboboxItem>
                    ))}
                  </ComboboxGroup>
                </ComboboxList>
              </ComboboxContent>
            </Combobox>
            {selectedWorkspaceLabel ? (
              <p className="px-1 text-xs text-muted-foreground">
                Selected: {selectedWorkspaceLabel}
              </p>
            ) : null}
          </div>
          <div className="flex items-center justify-start group-data-[collapsible=icon]:hidden">
            <div className="ml-1">
              <AnimatedTabs
                className="bg-card-tint-peach/50"
                activeTab={activeTab}
                layoutId="pill-demo"
                onChange={setActiveTab}
                tabs={tabs}
                variant="pill"
              />
            </div>
          </div>
        </SidebarHeader>
        <div className="border-b border-dashed border-border" />
        <SidebarContent>
          <NavSection
            items={mainMenuItems}
            pathname={pathname}
          />
          <NavSection
            label="Others"
            items={othersMenuItems}
            pathname={pathname}
          />

          {activeTab === "white-board" ? (
            <SidebarGroup>
              <SidebarGroupLabel className="text-[13px] uppercase text-muted-foreground">
                Recent Whiteboards
              </SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
                  {recentWhiteboards.slice(0, 5).map((wb) => (
                    <SidebarMenuItem key={wb.id}>
                      <Link href={wb.url}>
                        <SidebarMenuButton
                          isActive={pathname.startsWith(wb.url)}
                          tooltip={wb.title}
                          className="h-9 px-3 py-2 text-[13px] tracking-tight font-medium">
                          <span>{wb.title}</span>
                        </SidebarMenuButton>
                      </Link>
                    </SidebarMenuItem>
                  ))}
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          ) : (
            <SidebarGroup>
              <SidebarGroupLabel className="text-[13px] uppercase text-muted-foreground">
                Recent Notes
              </SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
                  {recentNotes.slice(0, 5).map((n) => (
                    <SidebarMenuItem key={n.id}>
                      <Link href={n.url}>
                        <SidebarMenuButton
                          isActive={pathname.startsWith(n.url)}
                          tooltip={n.title}
                          className="h-9 px-3 py-2 text-[13px] tracking-tight font-medium">
                          <span>{n.title}</span>
                        </SidebarMenuButton>
                      </Link>
                    </SidebarMenuItem>
                  ))}
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          )}
        </SidebarContent>
        <div className="border-b border-dashed border-border" />
        <SidebarFooter className="gap-3 py-3">
          {/*<UsageContainer />*/}
          <SidebarMenu>
            <SidebarMenuItem>
              <UserButton />
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarFooter>
        <SidebarRail />
      </Sidebar>
    </>
  )
}
