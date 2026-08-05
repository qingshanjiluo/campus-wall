// localStorage-backed registry of interrupted (started-but-not-completed)
// multipart toolset uploads, scoped per toolset. Lets the upload modal surface
// "you have N unfinished uploads" + a resume / delete list that survives a page
// reload — the artifact's already-uploaded parts live in B2, so a resume only
// re-sends the missing ones (see Upload.vue + the artifact /resume endpoint).
//
// All access is guarded (SSR / private mode / quota): failures degrade to an
// empty list rather than throwing, so the worst case is "resume isn't offered".
export const useToolsetResumeUploads = (toolsetId: number) => {
  const key = `kungal:toolset-upload-resume:${toolsetId}`

  const read = (): ToolsetPendingUpload[] => {
    if (!import.meta.client) {
      return []
    }
    try {
      const raw = localStorage.getItem(key)
      const parsed = raw ? JSON.parse(raw) : []
      return Array.isArray(parsed) ? (parsed as ToolsetPendingUpload[]) : []
    } catch {
      return []
    }
  }

  const write = (items: ToolsetPendingUpload[]) => {
    if (!import.meta.client) {
      return
    }
    try {
      localStorage.setItem(key, JSON.stringify(items))
    } catch {
      // private mode / quota — resume-across-reload just won't be offered
    }
  }

  // Newest first, so the list reads most-recent-on-top.
  const list = (): ToolsetPendingUpload[] =>
    read().sort((a, b) => b.updated_at - a.updated_at)

  // Insert or replace by uuid.
  const upsert = (record: ToolsetPendingUpload) => {
    write([
      ...read().filter((p) => p.artifact_uuid !== record.artifact_uuid),
      record
    ])
  }

  // Update the persisted progress as parts land — keeps the list accurate even
  // if the tab is closed mid-upload (no interruption event fires then).
  const setProgress = (artifactUuid: string, progress: number) => {
    const all = read()
    const record = all.find((p) => p.artifact_uuid === artifactUuid)
    if (!record) {
      return
    }
    record.progress = progress
    record.updated_at = Date.now()
    write(all)
  }

  const remove = (artifactUuid: string) => {
    write(read().filter((p) => p.artifact_uuid !== artifactUuid))
  }

  return { list, upsert, setProgress, remove }
}
