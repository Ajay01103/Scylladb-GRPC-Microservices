"use client"

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
  workspacesQueryOptions,
} from "@/modules/workspace/api/use-workspaces"
import { useSuspenseQuery } from "@tanstack/react-query"
import { type LucideIcon, LayoutGrid, Settings, Headphones, Users } from "lucide-react"
import Image from "next/image"
import Link from "next/link"
import { useParams, usePathname, useRouter } from "next/navigation"
import { useEffect, useMemo, useState, type KeyboardEvent } from "react"

interface MenuItem {
  title: string
  url?: string
  icon: LucideIcon
  onClick?: () => void
  match?: "exact" | "prefix"
}

interface NavSectionProps {
  label?: string
  items: MenuItem[]
  pathname: string
}

const NavSection = ({ label, items, pathname }: NavSectionProps) => {
  const isItemActive = (item: MenuItem) => {
    if (!item.url) {
      return false
    }

    if (item.match === "exact") {
      return pathname === item.url
    }

    return item.url === "/" ? pathname === "/" : pathname.startsWith(item.url)
  }

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
                    isActive={isItemActive(item)}
                    tooltip={item.title}
                    className="h-9 px-3 py-2 text-[13px] tracking-tight font-medium border border-transparent data-[active=true]:border-border data-[active=true]:shadow-[0px_1px_1px_0px_rgba(44,54,53,0.03),inset_0px_0px_0px_2px_white]"
                  >
                    <item.icon />
                    <span>{item.title}</span>
                  </SidebarMenuButton>
                </Link>
              ) : (
                <SidebarMenuButton
                  isActive={false}
                  onClick={item.onClick}
                  tooltip={item.title}
                  className="h-9 px-3 py-2 text-[13px] tracking-tight font-medium border border-transparent data-[active=true]:border-border data-[active=true]:shadow-[0px_1px_1px_0px_rgba(44,54,53,0.03),inset_0px_0px_0px_2px_white]"
                >
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
  const [workspaceSearch, setWorkspaceSearch] = useState("")
  const [workspaceMenuOpen, setWorkspaceMenuOpen] = useState(false)

  const { data: workspaces } = useSuspenseQuery(workspacesQueryOptions)
  const createWorkspaceMutation = useCreateWorkspace()

  const workspaceOptions = useMemo(
    () =>
      workspaces.map((workspace) => ({
        value: workspace.id,
        label: workspace.name,
        keywords: [workspace.name],
      })),
    [workspaces],
  )
  const defaultWorkspaceId = workspaceOptions[0]?.value ?? ""
  const effectiveSelectedWorkspaceId = routeWorkspaceId || defaultWorkspaceId
  const selectedWorkspaceLabel =
    workspaceOptions.find((workspace) => workspace.value === effectiveSelectedWorkspaceId)?.label ??
    ""

  const filteredWorkspaces = useMemo(() => {
    const normalizedSearch = workspaceSearch.trim().toLowerCase()

    if (!normalizedSearch) {
      return workspaceOptions
    }

    return workspaceOptions.filter((workspace) =>
      workspace.label.toLowerCase().includes(normalizedSearch),
    )
  }, [workspaceOptions, workspaceSearch])

  const workspaceNavigationItems: MenuItem[] = effectiveSelectedWorkspaceId
    ? [
        {
          title: "Overview",
          url: `/workspace/${effectiveSelectedWorkspaceId}`,
          icon: LayoutGrid,
          match: "exact",
        },
        {
          title: "Members",
          url: `/workspace/${effectiveSelectedWorkspaceId}/members`,
          icon: Users,
        },
      ]
    : []

  const othersMenuItems: MenuItem[] = [
    {
      title: "Settings",
      icon: Settings,
    },
    {
      title: "Help and support",
      url: "mailto:business@codewithantonio.com",
      icon: Headphones,
    },
  ]

  useEffect(() => {
    if (!workspaceMenuOpen) {
      setWorkspaceSearch("")
    }
  }, [workspaceMenuOpen])

  const handleWorkspaceChange = (workspaceId: string) => {
    if (!workspaceId) {
      return
    }

    setWorkspaceSearch("")
    setWorkspaceMenuOpen(false)
    router.push(`/workspace/${workspaceId}`)
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
            <Image src="/logo.svg" alt="Resonance" width={24} height={24} className="rounded-sm" />
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
              value={effectiveSelectedWorkspaceId}
            >
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
                        value={workspace.value}
                      >
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
          <div className="flex items-center justify-start group-data-[collapsible=icon]:hidden"></div>
        </SidebarHeader>
        <div className="border-b border-dashed border-border" />
        <SidebarContent>
          {workspaceNavigationItems.length > 0 ? (
            <NavSection label="Workspace" items={workspaceNavigationItems} pathname={pathname} />
          ) : null}
          <NavSection items={[]} pathname={pathname} />
          <NavSection label="Others" items={othersMenuItems} pathname={pathname} />
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
