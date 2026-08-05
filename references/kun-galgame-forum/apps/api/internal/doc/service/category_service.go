package service

import (
	"fmt"

	"kun-galgame-api/internal/doc/dto"
	"kun-galgame-api/internal/doc/model"
	"kun-galgame-api/internal/doc/repository"
	"kun-galgame-api/pkg/errors"
)

type CategoryService struct {
	categoryRepo *repository.CategoryRepository
}

func NewCategoryService(categoryRepo *repository.CategoryRepository) *CategoryService {
	return &CategoryService{categoryRepo: categoryRepo}
}

// CategoryListResult carries the list + total for paginated handler responses.
type CategoryListResult struct {
	Items []model.DocCategory
	Total int64
}

// GetList — GET /doc/category
func (s *CategoryService) GetList(req *dto.GetCategoriesRequest) *CategoryListResult {
	items, total := s.categoryRepo.FindPaginated(req.Keyword, req.Page, req.Limit)
	return &CategoryListResult{Items: items, Total: total}
}

// Create — POST /doc/category
func (s *CategoryService) Create(req *dto.CreateCategoryRequest) (*model.DocCategory, *errors.AppError) {
	category := &model.DocCategory{
		Slug:        req.Slug,
		Title:       req.Title,
		Description: req.Description,
		Icon:        req.Icon,
		SortOrder:   req.SortOrder,
	}
	if err := s.categoryRepo.Create(category); err != nil {
		return nil, errors.ErrInternal("创建分类失败")
	}
	return category, nil
}

// Update — PUT /doc/category
func (s *CategoryService) Update(req *dto.UpdateCategoryRequest) *errors.AppError {
	if err := s.categoryRepo.UpdateFields(req.CategoryID, map[string]any{
		"slug":        req.Slug,
		"title":       req.Title,
		"description": req.Description,
		"icon":        req.Icon,
		"sort_order":  req.SortOrder,
	}); err != nil {
		return errors.ErrInternal("更新分类失败")
	}
	return nil
}

// Delete — DELETE /doc/category
//
// Guards against the DB-level CASCADE: the legacy Prisma schema declared
// `doc_article.category_id` with `onDelete: Cascade`, so a raw DELETE
// here would silently take down every article in the category. Reject
// before that happens — admins must move / delete articles first.
func (s *CategoryService) Delete(categoryID int) *errors.AppError {
	if count := s.categoryRepo.CountArticles(categoryID); count > 0 {
		return errors.ErrBadRequest(
			fmt.Sprintf("该分类下还有 %d 篇文章, 请先移动或删除文章后再删除分类", count),
		)
	}
	if err := s.categoryRepo.DeleteByID(categoryID); err != nil {
		return errors.ErrInternal("删除分类失败")
	}
	return nil
}
