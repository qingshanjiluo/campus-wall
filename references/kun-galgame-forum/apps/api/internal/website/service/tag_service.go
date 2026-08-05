package service

import (
	"kun-galgame-api/internal/website/dto"
	"kun-galgame-api/internal/website/model"
	"kun-galgame-api/internal/website/repository"
	"kun-galgame-api/pkg/errors"
)

type TagService struct {
	tagRepo      *repository.TagRepository
	websiteRepo  *repository.WebsiteRepository
	categoryRepo *repository.CategoryRepository
	cdnBase      string
}

func NewTagService(
	tagRepo *repository.TagRepository,
	websiteRepo *repository.WebsiteRepository,
	categoryRepo *repository.CategoryRepository,
	cdnBase string,
) *TagService {
	return &TagService{
		tagRepo:      tagRepo,
		websiteRepo:  websiteRepo,
		categoryRepo: categoryRepo,
		cdnBase:      cdnBase,
	}
}

// ──────────────────────────────────────────
// GetAll — GET /website-tag
// ──────────────────────────────────────────

func (s *TagService) GetAll() []model.GalgameWebsiteTag {
	return s.tagRepo.FindAll()
}

// ──────────────────────────────────────────
// GetDetail — GET /website-tag/:name
// ──────────────────────────────────────────

func (s *TagService) GetDetail(name string, isSFW bool) (*dto.WebsiteTagDetailResponse, *errors.AppError) {
	tag, err := s.tagRepo.FindByName(name)
	if err != nil {
		return nil, errors.ErrNotFound("未找到该标签")
	}

	rels := s.tagRepo.FindRelationsByTagID(tag.ID)
	websiteIDs := make([]int, len(rels))
	for i, r := range rels {
		websiteIDs[i] = r.GalgameWebsiteID
	}

	websites := s.websiteRepo.FindByIDs(websiteIDs, isSFW)
	catMap := s.categoryRepo.FindNamesByIDs(collectCategoryIDs(websites))
	levelMap := s.tagRepo.LevelSumsByWebsiteIDs(collectWebsiteIDs(websites))
	cards := websiteCardsFromRows(websites, catMap, levelMap, s.cdnBase)

	return &dto.WebsiteTagDetailResponse{
		ID:           tag.ID,
		Name:         tag.Name,
		Label:        tag.Label,
		Level:        tag.Level,
		Description:  tag.Description,
		WebsiteCount: len(websites),
		Websites:     cards,
		Created:      tag.CreatedAt,
		Updated:      tag.UpdatedAt,
	}, nil
}

// ──────────────────────────────────────────
// Create — POST /website-tag
// ──────────────────────────────────────────

func (s *TagService) Create(req *dto.CreateWebsiteTagRequest) *errors.AppError {
	tag := &model.GalgameWebsiteTag{
		Name:        req.Name,
		Label:       req.Label,
		Description: req.Description,
		Level:       req.Level,
	}
	if err := s.tagRepo.Create(tag); err != nil {
		return errors.ErrInternal("创建标签失败")
	}
	return nil
}

// ──────────────────────────────────────────
// Update — PUT /website-tag
// ──────────────────────────────────────────

func (s *TagService) Update(req *dto.UpdateWebsiteTagRequest) *errors.AppError {
	s.tagRepo.UpdateFields(req.TagID, map[string]any{
		"name":        req.Name,
		"label":       req.Label,
		"description": req.Description,
		"level":       req.Level,
	})
	return nil
}

// ──────────────────────────────────────────
// Delete — DELETE /website-tag
// ──────────────────────────────────────────

func (s *TagService) Delete(id int) *errors.AppError {
	s.tagRepo.DeleteByID(id)
	return nil
}
