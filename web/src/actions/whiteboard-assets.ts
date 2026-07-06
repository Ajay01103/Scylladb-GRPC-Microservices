"use server"

import { S3Client, PutObjectCommand, GetObjectCommand } from "@aws-sdk/client-s3"
import { createClient } from "@connectrpc/connect"
import { createConnectTransport } from "@connectrpc/connect-web"
import { WhiteboardService } from "@/gen/pb/whiteboard/whiteboard_pb"
import { requireAccessTokenAction } from "./auth"

const S3_ENDPOINT = process.env.S3_ENDPOINT || "http://localhost:9000"
// Must match the bucket created by docker-compose rustfs-init (default: "uploads")
const S3_BUCKET = process.env.S3_BUCKET || "uploads"
// Public-facing URL the browser uses to load assets (same as S3_ENDPOINT in local dev)
const S3_PUBLIC_URL = process.env.NEXT_PUBLIC_S3_URL || S3_ENDPOINT
const WHITEBOARD_BASE_URL = process.env.NEXT_PUBLIC_WHITEBOARD_RPC_URL ?? "http://localhost:9093"

const s3Client = new S3Client({
  endpoint: S3_ENDPOINT,
  region: "us-east-1",
  credentials: {
    accessKeyId: process.env.AWS_ACCESS_KEY_ID || "rustfsadmin",
    secretAccessKey: process.env.AWS_SECRET_ACCESS_KEY || "rustfsadmin",
  },
  forcePathStyle: true,
})

async function getWhiteboardServerClient() {
  const token = await requireAccessTokenAction()
  const transport = createConnectTransport({
    baseUrl: WHITEBOARD_BASE_URL,
    interceptors: [
      (next) => async (req) => {
        req.header.set("Authorization", `Bearer ${token}`)
        return await next(req)
      },
    ],
  })
  return createClient(WhiteboardService, transport)
}

export async function uploadAssetAction(formData: FormData) {
  try {
    const file = formData.get("file") as File
    const boardId = formData.get("boardId") as string
    const workspaceId = formData.get("workspaceId") as string
    const assetId = formData.get("assetId") as string

    if (!file || !boardId || !workspaceId || !assetId) {
      throw new Error("Missing required upload parameters")
    }

    const buffer = Buffer.from(await file.arrayBuffer())
    const s3Key = `workspaces/${workspaceId}/boards/${boardId}/assets/${assetId}_${file.name}`

    // 1. Upload to S3
    await s3Client.send(
      new PutObjectCommand({
        Bucket: S3_BUCKET,
        Key: s3Key,
        Body: buffer,
        ContentType: file.type,
      }),
    )

    // 2. Register asset in whiteboard database via gRPC
    const client = await getWhiteboardServerClient()
    await client.registerAsset({
      assetId,
      boardId,
      workspaceId,
      name: file.name,
      mimeType: file.type,
      sizeBytes: BigInt(file.size),
      s3Key,
    })

    // Return the public-facing S3 URL so the browser can load the asset directly.
    // S3_PUBLIC_URL defaults to S3_ENDPOINT in local dev but can differ in production.
    const directSrc = `${S3_PUBLIC_URL}/${S3_BUCKET}/${s3Key}`

    return { success: true, s3Key, src: directSrc }
  } catch (error: any) {
    console.error("Failed to upload asset:", error)
    return { success: false, error: error.message || "Failed to upload asset" }
  }
}

export async function downloadAssetAction(s3Key: string) {
  try {
    const command = new GetObjectCommand({
      Bucket: S3_BUCKET,
      Key: s3Key,
    })
    const response = await s3Client.send(command)
    if (!response.Body) {
      throw new Error("Empty response body from S3")
    }

    const bytes = await response.Body.transformToByteArray()
    const base64 = Buffer.from(bytes).toString("base64")

    return {
      success: true,
      base64,
      mimeType: response.ContentType || "application/octet-stream",
    }
  } catch (error: any) {
    console.error("Failed to download asset:", error)
    return { success: false, error: error.message || "Failed to download asset" }
  }
}
