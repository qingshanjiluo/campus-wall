package service

import (
	"context"
	"io"

	"kun-galgame-api/internal/image/repository"
	"kun-galgame-api/pkg/catalogclient"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/imageclient"
)

const (
	MaxImageSize    = 10 * 1024 * 1024 // 10MB
	dailyImageLimit = 50
)

type ImageService struct {
	repo *repository.ImageRepository
	// imgCli is the image_service client for forum's OWN content images —
	// topic + message inline images (site=kungal). Galgame covers/screenshots
	// no longer go through it: they ride the catalog edit face (see
	// catalogClient) so the catalog owns all galgame image bytes. Nil-able:
	// when the credentials are unset, topic/message uploads return a clear
	// "未配置" error.
	imgCli *imageclient.Client
	// catalogClient carries galgame cover/screenshot uploads to the catalog
	// edit face (wave 169), which uploads under the catalog image identity —
	// so every galgame image byte is owned by the catalog scope its refping
	// keeps alive, not forum's site=kungal.
	catalogClient *catalogclient.Client
}

func NewImageService(
	repo *repository.ImageRepository,
	imgCli *imageclient.Client,
	catalogClient *catalogclient.Client,
) *ImageService {
	return &ImageService{repo: repo, imgCli: imgCli, catalogClient: catalogClient}
}

// UploadCoverResult is the image_service result returned to cover/banner/icon
// uploaders (doc / friend-link / website). Unlike topic/message (which return a
// `/image/<hash>` token for inline markdown), cover fields store the bare hash
// and resolve it to a CDN URL on read — so the FE needs both the hash (to store)
// and the url (to preview right away).
type UploadCoverResult struct {
	Hash      string `json:"hash"`
	URL       string `json:"url"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Thumbhash string `json:"thumbhash,omitempty"`
}

// UploadCoverImage uploads a cover/banner/icon through image_service under the
// `topic` preset (reused — a generic main-image pipeline, so no new infra preset
// is needed) and returns the hash + resolved URL + dims/thumbhash. The caller
// stores the hash on the target entity (e.g. doc_article.banner_image_hash).
func (s *ImageService) UploadCoverImage(ctx context.Context, userID int, r io.Reader, filename string) (*UploadCoverResult, *errors.AppError) {
	if s.imgCli == nil {
		return nil, errors.ErrBadRequest(
			"图片上传服务未配置 (KUN_IMAGE_CLIENT_ID / KUN_IMAGE_CLIENT_SECRET)",
		)
	}

	count, err := s.repo.GetDailyCount(userID)
	if err != nil {
		return nil, errors.ErrInternal("查询用户失败")
	}
	if count >= dailyImageLimit {
		return nil, errors.ErrBadRequest("今日图片上传次数已达上限")
	}

	res, uErr := s.imgCli.Upload(ctx, r, filename, "topic")
	if uErr != nil {
		if ie, ok := uErr.(*imageclient.Error); ok {
			return nil, errors.New(ie.Code, ie.Message, ie.StatusCode)
		}
		return nil, errors.ErrInternal("图片上传失败")
	}

	s.repo.IncrementDailyCount(userID)
	return &UploadCoverResult{
		Hash:      res.Hash,
		URL:       res.URL,
		Width:     res.Width,
		Height:    res.Height,
		Thumbhash: res.Thumbhash,
	}, nil
}

// UploadTopicImage routes a user's inline post image through image_service
// under the `topic` preset (WebP q77, ≤1920×1080, EXIF stripped — see infra
// configs/image_presets.yaml) and returns the domain-independent token
// `/image/<hash>`, which the editor inserts as the image src. The token is
// resolved to the real CDN URL at render time (markdown.resolveContentImageRef)
// and by the /image/:hash 302 fallback — so a future CDN/domain change is one
// config flip, never a rewrite of stored content (image_service contract).
//
// The kungal-local per-USER daily quota is kept on purpose: image_service's
// quota is per-SITE, so this still gives per-user fair-use limiting + a
// friendly message before we even hit image_service.
func (s *ImageService) UploadTopicImage(ctx context.Context, userID int, r io.Reader, filename string) (string, *errors.AppError) {
	if s.imgCli == nil {
		return "", errors.ErrBadRequest(
			"图片上传服务未配置 (KUN_IMAGE_CLIENT_ID / KUN_IMAGE_CLIENT_SECRET)",
		)
	}

	count, err := s.repo.GetDailyCount(userID)
	if err != nil {
		return "", errors.ErrInternal("查询用户失败")
	}
	if count >= dailyImageLimit {
		return "", errors.ErrBadRequest("今日图片上传次数已达上限")
	}

	res, uErr := s.imgCli.Upload(ctx, r, filename, "topic")
	if uErr != nil {
		// Forward image_service's structured error (preset denied, MIME, quota,
		// oversize, …) so the user sees the real reason; else generic.
		if ie, ok := uErr.(*imageclient.Error); ok {
			return "", errors.New(ie.Code, ie.Message, ie.StatusCode)
		}
		return "", errors.ErrInternal("图片上传失败")
	}

	s.repo.IncrementDailyCount(userID)
	return "/image/" + res.Hash, nil
}

// UploadMessageImage routes a chat/private-message inline image through
// image_service under the `message` preset (same global pipeline as topic:
// WebP q77, ≤1920×1080, EXIF stripped — see infra configs/image_presets.yaml)
// and returns the domain-independent token `/image/<hash>`. At render time the
// message markdown renderer resolves it to the CDN URL BEFORE sanitization, so
// it lands on an allow-listed host (see internal/infrastructure/markdown
// RenderInline + resolveContentImageRef) and survives, while arbitrary external
// URLs do not.
//
// Shares the per-USER daily image quota with topic uploads on purpose — it's a
// generic abuse cap, not per-feature accounting; image_service still applies
// its own per-SITE quota on top.
func (s *ImageService) UploadMessageImage(ctx context.Context, userID int, r io.Reader, filename string) (string, *errors.AppError) {
	if s.imgCli == nil {
		return "", errors.ErrBadRequest(
			"图片上传服务未配置 (KUN_IMAGE_CLIENT_ID / KUN_IMAGE_CLIENT_SECRET)",
		)
	}

	count, err := s.repo.GetDailyCount(userID)
	if err != nil {
		return "", errors.ErrInternal("查询用户失败")
	}
	if count >= dailyImageLimit {
		return "", errors.ErrBadRequest("今日图片上传次数已达上限")
	}

	res, uErr := s.imgCli.Upload(ctx, r, filename, "message")
	if uErr != nil {
		if ie, ok := uErr.(*imageclient.Error); ok {
			return "", errors.New(ie.Code, ie.Message, ie.StatusCode)
		}
		return "", errors.ErrInternal("图片上传失败")
	}

	s.repo.IncrementDailyCount(userID)
	return "/image/" + res.Hash, nil
}
