let currentNoteId: string | null = null
let currentWorkspaceId: string | null = null

export function setNoteAssetContext(noteId: string, workspaceId: string) {
  currentNoteId = noteId
  currentWorkspaceId = workspaceId
}

export function clearNoteAssetContext() {
  currentNoteId = null
  currentWorkspaceId = null
}

export function getNoteAssetContext() {
  return { noteId: currentNoteId, workspaceId: currentWorkspaceId }
}
