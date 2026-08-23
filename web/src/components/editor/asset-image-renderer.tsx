"use client"

import { useState, useEffect, type ReactNode } from "react"
import { downloadNoteAssetAction } from "@/actions/note-assets"
import { Skeleton } from "@/components/ui/skeleton"

const blobUrlCache = new Map<string, string>()

function isS3Key(src: string | null | undefined): boolean {
  return !!normalizeS3Key(src)
}

function normalizeS3Key(src: string | null | undefined): string | null {
  if (!src) return null
  const marker = "workspaces/"
  const markerIndex = src.indexOf(marker)
  return markerIndex >= 0 ? src.slice(markerIndex) : null
}

interface AssetImageRendererProps {
  attributes: Record<string, unknown>
  children: ReactNode
  element: {
    props?: {
      id?: string | null
      src?: string | null
      alt?: string | null
      sizes?: { width?: number; height?: number } | null
      fit?: "contain" | "cover" | "fill" | null
      bgColor?: string | null
      s3Key?: string | null
    }
  }
}

export function AssetImageRenderer({ attributes, children, element }: AssetImageRendererProps) {
  const { id, src, alt, sizes, fit, s3Key } = element.props ?? {}
  const normalizedSrcKey = normalizeS3Key(src)
  const storageKey = s3Key ? normalizeS3Key(s3Key) : normalizedSrcKey
  const key = id ?? storageKey ?? src

  const [blobUrl, setBlobUrl] = useState<string | null>(() => {
    if (!key) return null
    return blobUrlCache.get(key) ?? null
  })
  const [loading, setLoading] = useState(() => {
    if (!key) return false
    return !blobUrlCache.has(key)
  })

  useEffect(() => {
    if (!key || blobUrlCache.has(key)) return

    let cancelled = false

    async function resolve() {
      try {
        const res = await downloadNoteAssetAction(id ?? "", storageKey ?? undefined)
        if (cancelled) return
        if (res.success && res.base64) {
          const binary = atob(res.base64)
          const bytes = new Uint8Array(binary.length)
          for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
          const blob = new Blob([bytes], { type: res.mimeType })
          const url = URL.createObjectURL(blob)
          blobUrlCache.set(key!, url)
          setBlobUrl(url)
        }
      } catch (error) {
        console.error("Failed to resolve note asset", { id, storageKey, error })
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    resolve()
    return () => {
      cancelled = true
    }
  }, [key])

  const resolvedSrc = blobUrl ?? (src && !isS3Key(src) ? src : null)

  return (
    <div {...attributes} contentEditable={false}>
      {loading && (
        <Skeleton className="w-full rounded-md" style={{ height: sizes?.height ?? 200 }} />
      )}
      {!loading && resolvedSrc && (
        <img
          src={resolvedSrc}
          alt={alt ?? ""}
          width={sizes?.width ?? undefined}
          height={sizes?.height ?? undefined}
          style={{
            objectFit: fit ?? "cover",
            maxWidth: "100%",
            display: loading ? "none" : "block",
          }}
        />
      )}
      {!loading && !resolvedSrc && (
        <div className="flex items-center justify-center rounded-md border border-dashed bg-muted/50 p-8 text-sm text-muted-foreground">
          Image failed to load
        </div>
      )}
      {children}
    </div>
  )
}
