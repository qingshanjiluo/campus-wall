import type { ToolsetComment } from './toolset-comment'

export interface ToolsetCard {
  id: number
  name: string
  user: KunUser
  type: string
  platform: string
  language: string
  version: string
  view: number
  download: number
  comment_count: number
  practicality_avg: number | null
  resource_update_time: Date | string
}

export interface ToolsetDetail {
  id: number
  name: string
  content_html: string
  content_markdown: string
  type: string
  platform: string
  language: string
  version: string
  homepage: string[]
  download: number
  view: number
  user: KunUser
  aliases: string[]
  practicality_avg: number | null
  practicality_count: number
  resource_update_time: Date | string
  resource: ToolsetResource[]
  edited: Date | string | null
  created: Date | string
  updated: Date | string
  rating_counts: Record<number, number>
  comment_count: number
  comment_preview: ToolsetComment[]
  contributors: KunUser[]
}

export interface ToolsetRating {
  counts: {
    [x: number]: number
  }
  avg: number
  // BE returns `null` when the caller hasn't rated yet (PracticalityResponse.Mine *int).
  mine: number | null
}

// Server-driven init response from the artifact service (via kungal's BFF).
// When multipart is false the browser does one PUT to uploadUrl; otherwise it
// slices by partSize and PUTs each part to parts[i].url, collecting ETags.
export interface ToolsetUploadInitResponse {
  artifact_uuid: string
  multipart: boolean
  upload_url?: string
  part_size?: number
  parts?: {
    part_number: number
    url: string
  }[]
  expires_at: string
}

export interface ToolsetUploadCompleteResponse {
  artifact_uuid: string
  size: number
}

// Resume an interrupted multipart upload: uploadedParts are already stored in B2
// (skip them, reuse the etag at complete), parts are fresh presigned URLs for
// only the missing parts. Same shape as init so the multipart PUT loop is shared;
// a single-part upload comes back multipart=false + a fresh uploadUrl.
export interface ToolsetUploadResumeResponse {
  artifact_uuid: string
  multipart: boolean
  upload_url?: string
  part_size?: number
  parts?: {
    part_number: number
    url: string
  }[]
  uploaded_parts?: {
    part_number: number
    etag: string
    size: number
  }[]
  expires_at: string
}

// Result emitted from the S3 upload widget once a full upload (init → PUT →
// complete) succeeds. The artifact uuid binds the upload to a toolset_resource
// row at create time; size pre-fills the file size input.
export interface ToolsetUploadResult {
  artifact_uuid: string
  size: number
}

// A multipart upload started but not completed, persisted in localStorage per
// toolset so the upload modal can surface "you have unfinished uploads" across
// page reloads. The browser can't read a file by path, so resuming needs the
// user to re-select it (matched by size+lastModified, so a moved/renamed file
// still resumes); the uploaded parts live in B2 on the artifact side. name is
// display-only. progress is the last-known % (updated
// as parts upload + on interruption) so the resume list can show how far it got.
export interface ToolsetPendingUpload {
  artifact_uuid: string
  name: string
  size: number
  last_modified: number
  progress: number
  updated_at: number
}

export interface ToolsetResource {
  id: number
  type: string
  size: string
  download: number
  status: number
}

export interface ToolsetResourceDetail extends ToolsetResource {
  user: KunUser
  content: string
  code: string
  note: string
  password: string
  edited: Date | string | null
  created: Date | string
  updated: Date | string
}
