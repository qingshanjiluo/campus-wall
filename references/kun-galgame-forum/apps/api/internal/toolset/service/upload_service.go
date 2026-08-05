package service

import (
	"context"
	stderrors "errors"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"kun-galgame-api/internal/toolset/dto"
	userModel "kun-galgame-api/internal/user/model"
	"kun-galgame-api/pkg/artifactclient"
	"kun-galgame-api/pkg/errors"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────
// Constants
// ──────────────────────────────────────────

const (
	// MaxLargeFileSize is kungal's own per-file ceiling for toolset archives.
	// The artifact service enforces its own per-site max as an outer bound; this
	// is the (tighter) product limit kungal applies before reserving quota.
	MaxLargeFileSize = 2 * 1024 * 1024 * 1024 // 2GB

	// UserDailyUploadLimit is the per-user per-day total upload BYTE budget,
	// enforced server-side at upload init. Mirrors the frontend hint
	// apps/web/app/config/upload.ts USER_DAILY_UPLOAD_LIMIT — which, on its own,
	// a direct API caller could bypass.
	UserDailyUploadLimit = 100 * 1024 * 1024 // 100MB/day

	// uploadBytesPerMB scales the moemoepoint daily-budget bonus.
	uploadBytesPerMB = 1024 * 1024
)

var allowedArchiveExts = map[string]bool{
	".7z": true, ".zip": true, ".rar": true,
}

// ──────────────────────────────────────────
// Service
// ──────────────────────────────────────────

// UploadService brokers toolset archive uploads through the centralized artifact
// service. kungal keeps its own per-user quota + ext allow-list; the artifact
// service owns the S3 mechanics (presigned URLs, multipart, size verify, opaque
// keys). Bytes never pass through kungal — the browser PUTs straight to B2.
type UploadService struct {
	art *artifactclient.Client
	rdb *redis.Client
	db  *gorm.DB
}

func NewUploadService(art *artifactclient.Client, rdb *redis.Client, db *gorm.DB) *UploadService {
	return &UploadService{art: art, rdb: rdb, db: db}
}

// checkDailyUploadBudget rejects an upload that would push the user past their
// daily byte budget (100MB + moemoepoint·MB; moderators are bounded only by the
// per-file cap). Read against the committed daily total; the per-upload
// increment happens at Complete with the verified actual size. A missing state
// row (brand-new user) reads as 0. Soft quota: concurrent inits can each pass
// before any commits, but the per-file cap bounds the overshoot.
func (s *UploadService) checkDailyUploadBudget(userID int, incoming int64, canModerate bool) *errors.AppError {
	if canModerate {
		return nil
	}
	var state userModel.KungalUserState
	err := s.db.Select("daily_toolset_upload_bytes, moemoepoint").
		Where("user_id = ?", userID).First(&state).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return errors.ErrInternal("校验上传额度失败")
	}
	budget := int64(UserDailyUploadLimit) + int64(state.Moemoepoint)*uploadBytesPerMB
	if state.DailyToolsetUploadBytes+incoming > budget {
		return errors.ErrBadRequest("超出今日上传额度, 请明天再试")
	}
	return nil
}

// ──────────────────────────────────────────
// Init — POST /toolset/:id/upload/init
// ──────────────────────────────────────────

// Init validates the file, reserves nothing locally, and asks the artifact
// service for presigned upload URL(s). The response is server-driven: single
// PUT or multipart (the frontend obeys whichever it gets).
func (s *UploadService) Init(
	ctx context.Context,
	toolsetID, userID int,
	canModerate bool,
	req *dto.UploadInitRequest,
) (*dto.UploadInitResponse, *errors.AppError) {
	if req.FileSize > MaxLargeFileSize {
		return nil, errors.ErrBadRequest("文件大小不能超过 2GB")
	}
	if _, _, appErr := parseArchiveFilename(req.Filename); appErr != nil {
		return nil, appErr
	}
	if appErr := s.checkDailyUploadBudget(userID, req.FileSize, canModerate); appErr != nil {
		return nil, appErr
	}

	public := true
	initReq := artifactclient.InitUploadRequest{
		Name:     req.Filename,
		FileSize: req.FileSize,
		Public:   &public,
	}
	if req.ContentType != "" {
		mime := req.ContentType
		initReq.MimeType = &mime
	}

	out, err := s.art.InitUpload(ctx, initReq)
	if err != nil {
		return nil, mapArtifactErr(err)
	}

	resp := &dto.UploadInitResponse{
		ArtifactUUID: out.Uuid,
		Multipart:    out.Multipart,
		ExpiresAt:    out.ExpiresAt,
	}
	if out.Multipart {
		if out.PartSize != nil {
			resp.PartSize = *out.PartSize
		}
		if out.PartUrls != nil {
			for _, p := range *out.PartUrls {
				resp.Parts = append(resp.Parts, dto.UploadInitPart{
					PartNumber: int(p.PartNumber),
					URL:        p.Url,
				})
			}
		}
	} else if out.UploadUrl != nil {
		resp.UploadURL = *out.UploadUrl
	}
	return resp, nil
}

// ──────────────────────────────────────────
// Complete — POST /toolset/:id/upload/complete
// ──────────────────────────────────────────

func (s *UploadService) Complete(
	ctx context.Context,
	userID int,
	req *dto.UploadCompleteRequest,
) (*dto.UploadCompleteResponse, *errors.AppError) {
	var parts *[]artifactclient.CompletedPart
	if len(req.Parts) > 0 {
		cps := make([]artifactclient.CompletedPart, len(req.Parts))
		for i, p := range req.Parts {
			cps[i] = artifactclient.CompletedPart{Etag: p.ETag, PartNumber: p.PartNumber}
		}
		parts = &cps
	}

	art, err := s.art.CompleteUpload(ctx, req.ArtifactUUID, artifactclient.CompleteUploadRequest{Parts: parts})
	if err != nil {
		return nil, mapArtifactErr(err)
	}

	// Accrue the per-user daily count + byte budget exactly once per artifact
	// (a retried complete must not double-count). The artifact service already
	// verified the real size via HeadObject, so trust art.FileSize.
	if s.firstComplete(ctx, art.Uuid) {
		s.db.Model(&userModel.KungalUserState{}).Where("user_id = ?", userID).
			Updates(map[string]any{
				"daily_toolset_upload_count": gorm.Expr("daily_toolset_upload_count + 1"),
				"daily_toolset_upload_bytes": gorm.Expr("daily_toolset_upload_bytes + ?", art.FileSize),
			})
	}

	return &dto.UploadCompleteResponse{ArtifactUUID: art.Uuid, Size: art.FileSize}, nil
}

// ──────────────────────────────────────────
// Resume — POST /toolset/:id/upload/resume
// ──────────────────────────────────────────

// Resume continues an interrupted multipart upload: it asks the artifact service
// which parts are already stored (skip them) and returns fresh presigned URLs for
// only the missing parts, so a paused / dropped / page-refreshed upload finishes
// without re-sending bytes already in B2. No quota is touched here — the daily
// budget is reserved at Init and accrued once at Complete; calling resume also
// refreshes the artifact's activity timestamp so GC won't reap it mid-resume.
func (s *UploadService) Resume(
	ctx context.Context,
	req *dto.UploadResumeRequest,
) (*dto.UploadResumeResponse, *errors.AppError) {
	out, err := s.art.Resume(ctx, req.ArtifactUUID)
	if err != nil {
		return nil, mapArtifactErr(err)
	}

	resp := &dto.UploadResumeResponse{
		ArtifactUUID: out.Uuid,
		Multipart:    out.Multipart,
		ExpiresAt:    out.ExpiresAt,
	}
	if out.Multipart {
		if out.PartSize != nil {
			resp.PartSize = *out.PartSize
		}
		if out.PartUrls != nil {
			for _, p := range *out.PartUrls {
				resp.Parts = append(resp.Parts, dto.UploadInitPart{
					PartNumber: int(p.PartNumber),
					URL:        p.Url,
				})
			}
		}
		if out.UploadedParts != nil {
			for _, p := range *out.UploadedParts {
				resp.UploadedParts = append(resp.UploadedParts, dto.UploadResumePart{
					PartNumber: int(p.PartNumber),
					ETag:       p.Etag,
					Size:       p.Size,
				})
			}
		}
	} else if out.UploadUrl != nil {
		resp.UploadURL = *out.UploadUrl
	}
	return resp, nil
}

// ──────────────────────────────────────────
// Abort — POST /toolset/:id/upload/abort
// ──────────────────────────────────────────

// Abort soft-deletes an unfinished artifact (GC reclaims it). Best-effort.
func (s *UploadService) Abort(ctx context.Context, req *dto.UploadAbortRequest) *errors.AppError {
	if err := s.art.Delete(ctx, req.ArtifactUUID); err != nil {
		slog.Warn("取消上传失败", "uuid", req.ArtifactUUID, "error", err)
	}
	return nil
}

// ──────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────

// firstComplete returns true the first time complete runs for a uuid, so the
// daily-budget accrual happens exactly once even if the client retries. Redis
// being down fails toward accruing (charge the quota) rather than free uploads.
func (s *UploadService) firstComplete(ctx context.Context, uuid string) bool {
	ok, err := s.rdb.SetNX(ctx, "toolset:upload:done:"+uuid, 1, 24*time.Hour).Result()
	if err != nil {
		return true
	}
	return ok
}

// parseArchiveFilename validates the extension against the allow-list. The base
// name is no longer used for keying (the artifact service assigns opaque keys),
// but kept in the return for call-site clarity.
func parseArchiveFilename(filename string) (ext, base string, appErr *errors.AppError) {
	ext = strings.ToLower(filepath.Ext(filename))
	if !allowedArchiveExts[ext] {
		return "", "", errors.ErrBadRequest("仅支持 .7z, .zip, .rar 格式")
	}
	base = strings.TrimSuffix(filename, ext)
	return ext, base, nil
}

// mapArtifactErr translates an artifactclient sentinel into a kungal AppError.
func mapArtifactErr(err error) *errors.AppError {
	switch {
	case stderrors.Is(err, artifactclient.ErrTooBig):
		return errors.ErrBadRequest("文件大小超过限制")
	case stderrors.Is(err, artifactclient.ErrQuotaExceeded):
		return errors.ErrBadRequest("超出上传额度, 请稍后再试")
	case stderrors.Is(err, artifactclient.ErrMIMEDenied):
		return errors.ErrBadRequest("仅支持 .7z, .zip, .rar 格式")
	case stderrors.Is(err, artifactclient.ErrSizeMismatch):
		return errors.ErrBadRequest("文件大小与声明不符, 上传已被拒绝")
	case stderrors.Is(err, artifactclient.ErrUploadDisabled):
		return errors.ErrBadRequest("上传服务暂时不可用")
	case stderrors.Is(err, artifactclient.ErrNotConfigured):
		return errors.ErrInternal("上传服务未配置")
	case stderrors.Is(err, artifactclient.ErrNotFound):
		return errors.ErrNotFound("上传会话不存在或已过期")
	default:
		return errors.ErrInternal("上传服务请求失败")
	}
}
