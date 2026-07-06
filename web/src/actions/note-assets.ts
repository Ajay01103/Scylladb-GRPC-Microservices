"use server"

import { S3Client, PutObjectCommand, GetObjectCommand } from "@aws-sdk/client-s3"
import { createClient } from "@connectrpc/connect"
import { createConnectTransport } from "@connectrpc/connect-web"
import { NotesService } from "@/gen/pb/notes/notes_pb"
import { requireAccessTokenAction } from "./auth"

const S3_ENDPOINT = process.env.S3_ENDPOINT || "http://localhost:9000"
const S3_BUCKET = process.env.S3_BUCKET || "uploads"
const NOTES_BASE_URL = process.env.NEXT_PUBLIC_NOTES_RPC_URL ?? "http://localhost:9092"

const s3Client = new S3Client({
  endpoint: S3_ENDPOINT,
  region: "us-east-1",
  credentials: {
    accessKeyId: process.env.AWS_ACCESS_KEY_ID || "rustfsadmin",
    secretAccessKey: process.env.AWS_SECRET_ACCESS_KEY || "rustfsadmin",
  },
  forcePathStyle: true,
})

async function getNotesServerClient() {
  const token = await requireAccessTokenAction()
  const transport = createConnectTransport({
    baseUrl: NOTES_BASE_URL,
    interceptors: [
      (next) => async (req) => {
        req.header.set("Authorization", `Bearer ${token}`)
        return await next(req)
      },
    ],
  })
  return createClient(NotesService, transport)
}

export async function uploadNoteAssetAction(formData: FormData) {
  try {
    const file = formData.get("file") as File
    const noteId = formData.get("noteId") as string
    const workspaceId = formData.get("workspaceId") as string
    const assetId = formData.get("assetId") as string

    if (!file || !noteId || !workspaceId || !assetId) {
      throw new Error("Missing required upload parameters")
    }

    const buffer = Buffer.from(await file.arrayBuffer())
    const s3Key = `workspaces/${workspaceId}/notes/${noteId}/assets/${assetId}_${file.name}`

    await s3Client.send(
      new PutObjectCommand({
        Bucket: S3_BUCKET,
        Key: s3Key,
        Body: buffer,
        ContentType: file.type,
      }),
    )

    const client = await getNotesServerClient()
    await client.registerNoteAsset({
      assetId,
      noteId,
      name: file.name,
      mimeType: file.type,
      sizeBytes: BigInt(file.size),
      s3Key,
    })

    const { downloadUrl } = await client.getAssetDownloadUrl({ assetId })
    return { success: true, assetId, s3Key, src: downloadUrl }
  } catch (error: any) {
    console.error("Failed to upload note asset:", error)
    return { success: false, error: error.message || "Failed to upload note asset" }
  }
}

export async function downloadNoteAssetAction(s3Key: string) {
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
    console.error("Failed to download note asset:", error)
    return { success: false, error: error.message || "Failed to download note asset" }
  }
}
