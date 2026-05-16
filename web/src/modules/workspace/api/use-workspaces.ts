"use client"

import { create } from "@bufbuild/protobuf"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { workspaceRpcClient } from "@/lib/rpc"
import { useIsLoadingAuth } from "@/lib/token-store"
import {
  CreateWorkspaceRequestSchema,
  type Workspace,
} from "@/gen/pb/workspace/workspace_pb"

export const workspaceQueryKey = ["myWorkspaces"] as const

export type WorkspaceOption = Pick<Workspace, "id" | "name" | "slug" | "description">

export function buildWorkspaceSlug(name: string) {
  const normalized = name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")

  return normalized || "workspace"
}

export function useMyWorkspaces() {
  const isLoadingAuth = useIsLoadingAuth()

  return useQuery({
    queryKey: workspaceQueryKey,
    enabled: !isLoadingAuth,
    staleTime: 5 * 60 * 1000,
    gcTime: 30 * 60 * 1000,
    refetchOnWindowFocus: false,
    retry: 1,
    queryFn: async (): Promise<Workspace[]> => {
      const response = await workspaceRpcClient.listMyWorkspaces({
        pageSize: 100,
        pageToken: "",
      })

      return response.workspaces
    },
  })
}

type CreateWorkspaceInput = {
  name: string
  slug: string
  description: string
  iconUrl: string
  isPublic: boolean
}

export function useCreateWorkspace() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (input: CreateWorkspaceInput) => {
      return workspaceRpcClient.createWorkspace(
        create(CreateWorkspaceRequestSchema, input),
      )
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: workspaceQueryKey })
    },
  })
}
