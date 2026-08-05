package service

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"kun-galgame-api/internal/infrastructure/storage"
	"kun-galgame-api/internal/moemoepoint"
	"kun-galgame-api/internal/toolset/dto"
	"kun-galgame-api/internal/toolset/model"
	"kun-galgame-api/internal/toolset/repository"
	"kun-galgame-api/internal/trust/gate"
	userModel "kun-galgame-api/internal/user/model"
	"kun-galgame-api/pkg/artifactclient"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/userclient"

	"gorm.io/gorm"
)

type ResourceService struct {
	resourceRepo *repository.ResourceRepository
	toolsetRepo  *repository.ToolsetRepository
	s3           *storage.S3Client
	art          *artifactclient.Client
	userClient   *userclient.Client
	check        *gate.CheckService
	scan         *gate.ScanService
}

func NewResourceService(
	resourceRepo *repository.ResourceRepository,
	toolsetRepo *repository.ToolsetRepository,
	s3 *storage.S3Client,
	art *artifactclient.Client,
	userClient *userclient.Client,
	check *gate.CheckService,
	scan *gate.ScanService,
) *ResourceService {
	return &ResourceService{
		resourceRepo: resourceRepo,
		toolsetRepo:  toolsetRepo,
		s3:           s3,
		art:          art,
		userClient:   userClient,
		check:        check,
		scan:         scan,
	}
}

// ──────────────────────────────────────────
// GetResourceDetail — GET /toolset/:id/resource/detail
// ──────────────────────────────────────────

func (s *ResourceService) GetResourceDetail(
	ctx context.Context,
	req *dto.ResourceDetailRequest,
) (*dto.ResourceDetailResponse, *errors.AppError) {
	resource, err := s.resourceRepo.FindByID(req.ResourceID)
	if err != nil {
		return nil, errors.ErrNotFound("未找到该资源")
	}

	// A banned owner's resource is hidden even by direct link — 404 before
	// counting a download or resolving the artifact URL.
	uc, _, _ := s.userClient.User(ctx, resource.UserID)
	if !userclient.IsRenderable(uc) {
		return nil, errors.ErrNotFound("未找到该资源")
	}

	// Download +1 (fire-and-forget)
	go s.resourceRepo.IncrementDownload(resource.ID)

	// Dual-read: a new s3 resource stores only the artifact uuid; resolve its
	// download URL at read time via the artifact service (served from
	// dl.imoe.uk). Legacy s3 rows keep their stored Content URL; on a transient
	// artifact error we leave Content as-is rather than fail the whole detail.
	if resource.Type == "s3" && resource.ArtifactUUID != "" {
		if dl, derr := s.art.Download(ctx, resource.ArtifactUUID); derr == nil {
			resource.Content = dl.Url
		} else {
			slog.Warn("解析 artifact 下载链接失败", "uuid", resource.ArtifactUUID, "error", derr)
		}
	}

	user := userModel.UserBrief{ID: uc.ID, Name: uc.Name, Avatar: uc.Avatar}

	return &dto.ResourceDetailResponse{
		GalgameToolsetResource: *resource,
		User:                   user,
	}, nil
}

// ──────────────────────────────────────────
// CreateResource — POST /toolset/:id/resource
// ──────────────────────────────────────────

func (s *ResourceService) CreateResource(
	ctx context.Context,
	userID, toolsetID int,
	req *dto.CreateResourceRequest,
) (*dto.CreatedResourceResponse, *errors.AppError) {
	// Verify toolset exists
	if _, err := s.toolsetRepo.FindByID(toolsetID); err != nil {
		return nil, errors.ErrNotFound("未找到该工具")
	}

	// Synchronous word-list gate BEFORE the tx (trust wave 2): content + note.
	// deny blocks (nothing persisted); hold publishes+logs; fail-open on error.
	moderationText := gate.ComposeText(req.Content, req.Note)
	authorID := int64(userID)
	decision, matched := s.check.Decision(ctx, moderationText, &authorID)
	if decision == gate.DecisionDeny {
		return nil, gate.ErrContentBlocked()
	}

	var resource model.GalgameToolsetResource
	txErr := s.resourceRepo.DB().Transaction(func(tx *gorm.DB) error {
		resource = model.GalgameToolsetResource{
			Content:      req.Content,
			Type:         req.Type,
			ArtifactUUID: req.ArtifactUUID,
			Code:         req.Code,
			Password:     req.Password,
			Size:         req.Size,
			Note:         req.Note,
			ToolsetID:    toolsetID,
			UserID:       userID,
		}
		if err := s.resourceRepo.Create(tx, &resource); err != nil {
			return err
		}

		// Moemoepoint +3 — stable key per created resource so an HTTP retry
		// dedups instead of double-awarding.
		adjustMoemoepoint(tx, userID, 3,
			moemoepoint.ReasonContentApproved, moemoepoint.Ref("toolset", toolsetID),
			moemoepoint.Key("resource_create", strconv.Itoa(resource.ID)))

		// Add contributor (ignore duplicate)
		s.toolsetRepo.AddContributor(tx, toolsetID, userID)

		// Refresh resource_update_time
		s.toolsetRepo.UpdateResourceTime(tx, toolsetID, time.Now())

		return nil
	})
	if txErr != nil {
		return nil, errors.ErrInternal("创建资源失败")
	}

	if decision == gate.DecisionHold {
		slog.Info("trust check hold", "subject_kind", gate.SubjectKindToolsetResource, "subject_id", resource.ID, "author_id", userID, "matched", matched)
	}
	s.scan.ScanBg(gate.SubjectKindToolsetResource, strconv.Itoa(resource.ID), moderationText, int64(userID))

	return &resource, nil
}

// ──────────────────────────────────────────
// UpdateResource — PUT /toolset/:id/resource
// ──────────────────────────────────────────

func (s *ResourceService) UpdateResource(
	ctx context.Context,
	userID int, canModerate bool,
	req *dto.UpdateResourceRequest,
) (*model.GalgameToolsetResource, *errors.AppError) {
	resource, err := s.resourceRepo.FindByID(req.ResourceID)
	if err != nil {
		return nil, errors.ErrNotFound("未找到该资源")
	}

	if resource.UserID != userID && !canModerate {
		return nil, errors.ErrForbidden("您没有权限编辑此资源")
	}

	// Synchronous word-list gate BEFORE the write (deny blocks, hold publishes+
	// logs, fail-open). author_id is the content author (resource.UserID).
	moderationText := gate.ComposeText(req.Content, req.Note)
	authorID := int64(resource.UserID)
	decision, matched := s.check.Decision(ctx, moderationText, &authorID)
	if decision == gate.DecisionDeny {
		return nil, gate.ErrContentBlocked()
	}

	now := time.Now()
	updates := map[string]any{
		"password": req.Password,
		"note":     req.Note,
		"edited":   now,
	}

	// S3 type: only password and note can be updated.
	// User type: all fields can be updated.
	if resource.Type == "user" {
		updates["content"] = req.Content
		updates["code"] = req.Code
		updates["size"] = req.Size
	}

	s.resourceRepo.UpdateFields(resource, updates)

	// Re-read after the update so the returned row carries the fresh
	// `edited` timestamp and any DB-side defaults. The frontend assigns
	// this directly into the resource list item, so handing back stale
	// fields would resurrect the NaN-time / undefined-link symptoms we
	// just fixed.
	refreshed, refreshErr := s.resourceRepo.FindByID(resource.ID)
	if refreshErr != nil {
		return nil, errors.ErrInternal("读取更新后的资源失败")
	}

	if decision == gate.DecisionHold {
		slog.Info("trust check hold", "subject_kind", gate.SubjectKindToolsetResource, "subject_id", req.ResourceID, "author_id", resource.UserID, "matched", matched)
	}
	s.scan.ScanBg(gate.SubjectKindToolsetResource, strconv.Itoa(req.ResourceID), moderationText, int64(resource.UserID))
	return refreshed, nil
}

// ──────────────────────────────────────────
// DeleteResource — DELETE /toolset/:id/resource
// ──────────────────────────────────────────

func (s *ResourceService) DeleteResource(
	userID int, canModerate bool,
	req *dto.DeleteResourceRequest,
) *errors.AppError {
	resource, err := s.resourceRepo.FindByID(req.ResourceID)
	if err != nil {
		return errors.ErrNotFound("未找到该资源")
	}

	if resource.UserID != userID && !canModerate {
		return errors.ErrForbidden("您没有权限删除此资源")
	}

	// Storage cleanup (best-effort): new s3 rows soft-delete via the artifact
	// service (GC reclaims the bytes); legacy s3 rows still carry their key in
	// Code and are deleted from the old bucket directly.
	if resource.Type == "s3" {
		if resource.ArtifactUUID != "" {
			if err := s.art.Delete(context.Background(), resource.ArtifactUUID); err != nil {
				slog.Warn("删除 artifact 资源失败", "uuid", resource.ArtifactUUID, "error", err)
			}
		} else if resource.Code != "" && s.s3 != nil {
			if err := s.s3.Delete(context.Background(), resource.Code); err != nil {
				slog.Warn("删除 S3 资源失败", "key", resource.Code, "error", err)
			}
		}
	}

	s.resourceRepo.Delete(resource)

	// Moemoepoint -3 on the resource owner — stable key per deleted resource.
	adjustMoemoepoint(s.resourceRepo.DB(), resource.UserID, -3,
		moemoepoint.ReasonContentRemoved, moemoepoint.Ref("toolset_resource", resource.ID),
		moemoepoint.Key("resource_delete", strconv.Itoa(resource.ID)))

	return nil
}

// ──────────────────────────────────────────
// Shared helpers
// ──────────────────────────────────────────

// adjustMoemoepoint applies a moemoepoint change via the OAuth single source
// (terminal state — NO local +=). The Awarder mirrors the authoritative
// balance into kungal_user_state; async/best-effort, never blocks. `db` is
// unused (kept for call-style uniformity); the award doesn't touch the caller's
// transaction.
func adjustMoemoepoint(_ *gorm.DB, userID, delta int, reason, ref, idempotencyKey string) {
	moemoepoint.Award(userID, delta, reason, ref, idempotencyKey)
}
