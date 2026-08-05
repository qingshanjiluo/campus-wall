package dto

// Toolset archive upload is brokered by the centralized artifact service
// (kun-galgame-infra). The browser PUTs bytes straight to B2 via presigned URLs
// the artifact service returns; kungal only drives the init/complete/abort JSON
// calls and keeps its own per-user quota + ext allow-list. The flow is
// server-driven: init returns either a single upload URL or a set of multipart
// part URLs (with part_size) — the frontend obeys whichever it gets.

// ──────────────────────────────────────────
// Requests
// ──────────────────────────────────────────

type UploadInitRequest struct {
	Filename    string `json:"filename" validate:"required"`
	FileSize    int64  `json:"filesize" validate:"required,min=1"`
	ContentType string `json:"content_type"`
}

type UploadCompletePart struct {
	PartNumber int32  `json:"part_number"`
	ETag       string `json:"etag"`
}

type UploadCompleteRequest struct {
	ArtifactUUID string               `json:"artifact_uuid" validate:"required"`
	Parts        []UploadCompletePart `json:"parts"` // multipart only
}

type UploadAbortRequest struct {
	ArtifactUUID string `json:"artifact_uuid" validate:"required"`
}

type UploadResumeRequest struct {
	ArtifactUUID string `json:"artifact_uuid" validate:"required"`
}

// ──────────────────────────────────────────
// Responses
// ──────────────────────────────────────────

type UploadInitPart struct {
	PartNumber int    `json:"part_number"`
	URL        string `json:"url"`
}

// UploadInitResponse mirrors the artifact service's init response. When
// Multipart is false the browser does one PUT to UploadURL; otherwise it slices
// by PartSize and PUTs each part to Parts[i].URL, collecting ETags for complete.
type UploadInitResponse struct {
	ArtifactUUID string           `json:"artifact_uuid"`
	Multipart    bool             `json:"multipart"`
	UploadURL    string           `json:"upload_url,omitempty"`
	PartSize     int64            `json:"part_size,omitempty"`
	Parts        []UploadInitPart `json:"parts,omitempty"`
	ExpiresAt    string           `json:"expires_at"`
}

type UploadCompleteResponse struct {
	ArtifactUUID string `json:"artifact_uuid"`
	Size         int64  `json:"size"`
}

// UploadResumePart is a part already stored on the artifact side — the frontend
// skips re-uploading it and reuses ETag at complete.
type UploadResumePart struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
	Size       int64  `json:"size"`
}

// UploadResumeResponse lets the frontend continue an interrupted upload without
// starting over: UploadedParts are already stored (skip + reuse ETag), Parts are
// fresh presigned URLs for only the missing parts. Mirrors UploadInitResponse so
// the frontend's multipart loop is identical. For a single-part upload Multipart
// is false and the whole file is re-PUT to UploadURL.
type UploadResumeResponse struct {
	ArtifactUUID  string             `json:"artifact_uuid"`
	Multipart     bool               `json:"multipart"`
	UploadURL     string             `json:"upload_url,omitempty"`
	PartSize      int64              `json:"part_size,omitempty"`
	Parts         []UploadInitPart   `json:"parts,omitempty"`
	UploadedParts []UploadResumePart `json:"uploaded_parts,omitempty"`
	ExpiresAt     string             `json:"expires_at"`
}
