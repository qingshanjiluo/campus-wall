package repository

import (
	"kun-galgame-api/internal/doc/dto"
	"kun-galgame-api/internal/doc/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ArticleRepository struct {
	db *gorm.DB
}

func NewArticleRepository(db *gorm.DB) *ArticleRepository {
	return &ArticleRepository{db: db}
}

func (r *ArticleRepository) DB() *gorm.DB { return r.db }

// ──────────────────────────────────────────
// Reads
// ──────────────────────────────────────────

// FindPaginated applies the filters from GetArticlesRequest and returns the
// matching articles plus total count.
func (r *ArticleRepository) FindPaginated(req *dto.GetArticlesRequest) ([]model.DocArticle, int64) {
	query := r.db.Model(&model.DocArticle{})

	// Default: only published articles. AllStatuses (admin manager only) returns
	// every status; an explicit Status filters to that one.
	if req.AllStatuses {
		// no status filter
	} else if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	} else {
		query = query.Where("status = 1")
	}
	if req.CategoryID != nil {
		query = query.Where("category_id = ?", *req.CategoryID)
	}
	if req.IsPin != nil {
		query = query.Where("is_pin = ?", *req.IsPin)
	}
	if req.Keyword != "" {
		query = query.Where(
			"title ILIKE ? OR slug ILIKE ?",
			"%"+req.Keyword+"%", "%"+req.Keyword+"%",
		)
	}
	if req.TagID != nil {
		query = query.Where(
			"id IN (SELECT doc_article_id FROM doc_article_tag_relation WHERE doc_tag_id = ?)",
			*req.TagID,
		)
	}

	var total int64
	query.Count(&total)

	var articles []model.DocArticle
	query.Order(req.OrderBy + " " + req.SortOrder).
		Order("id ASC").
		Offset((req.Page - 1) * req.Limit).Limit(req.Limit).
		Find(&articles)

	return articles, total
}

// FindBySlug returns a single article by slug.
func (r *ArticleRepository) FindBySlug(slug string) (*model.DocArticle, error) {
	var article model.DocArticle
	if err := r.db.Where("slug = ?", slug).First(&article).Error; err != nil {
		return nil, err
	}
	return &article, nil
}

// IncrementView bumps view by 1 (fire-and-forget).
func (r *ArticleRepository) IncrementView(id int) {
	r.db.Model(&model.DocArticle{}).Where("id = ?", id).
		Update("view", gorm.Expr("view + 1"))
}

// ──────────────────────────────────────────
// Writes
// ──────────────────────────────────────────

// Create inserts a new article row (inside a tx).
func (r *ArticleRepository) Create(tx *gorm.DB, article *model.DocArticle) error {
	return tx.Create(article).Error
}

// UpdateFields updates arbitrary fields on an article row (inside a tx).
func (r *ArticleRepository) UpdateFields(tx *gorm.DB, id int, updates map[string]any) error {
	return tx.Model(&model.DocArticle{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteByID deletes an article row.
func (r *ArticleRepository) DeleteByID(id int) error {
	return r.db.Delete(&model.DocArticle{}, id).Error
}

// Reorder rewrites sort_order for every id to its index in the provided slice
// (the drag-reorder result, top → bottom), in one transaction. A stray id
// (e.g. already deleted) updates zero rows and is silently skipped.
func (r *ArticleRepository) Reorder(ids []int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for i, id := range ids {
			if err := tx.Model(&model.DocArticle{}).
				Where("id = ?", id).
				Update("sort_order", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// SetPin updates only the is_pin flag of one article (the quick pin toggle in
// the admin doc manager).
func (r *ArticleRepository) SetPin(id int, isPin bool) error {
	return r.db.Model(&model.DocArticle{}).
		Where("id = ?", id).
		Update("is_pin", isPin).Error
}

// ReplaceTagRelations deletes existing tag relations and inserts the new set
// atomically inside the given tx.
func (r *ArticleRepository) ReplaceTagRelations(tx *gorm.DB, articleID int, tagIDs []int) error {
	if err := tx.Where("doc_article_id = ?", articleID).
		Delete(&model.DocArticleTagRelation{}).Error; err != nil {
		return err
	}
	for _, tagID := range tagIDs {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.DocArticleTagRelation{
			DocArticleID: articleID, DocTagID: tagID,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

// InsertTagRelations inserts tag relations for a newly created article.
func (r *ArticleRepository) InsertTagRelations(tx *gorm.DB, articleID int, tagIDs []int) error {
	for _, tagID := range tagIDs {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.DocArticleTagRelation{
			DocArticleID: articleID, DocTagID: tagID,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

// DeleteTagRelationsByArticleID deletes all tag relations for an article.
func (r *ArticleRepository) DeleteTagRelationsByArticleID(articleID int) error {
	return r.db.Where("doc_article_id = ?", articleID).
		Delete(&model.DocArticleTagRelation{}).Error
}

// FindTagIDsByArticleID returns the list of tag IDs attached to an
// article. Always returns a non-nil slice (empty when no tags) so the
// JSON response is `[]` rather than `null`.
func (r *ArticleRepository) FindTagIDsByArticleID(articleID int) ([]int, error) {
	var ids []int
	if err := r.db.Model(&model.DocArticleTagRelation{}).
		Where("doc_article_id = ?", articleID).
		Pluck("doc_tag_id", &ids).Error; err != nil {
		return nil, err
	}
	if ids == nil {
		return []int{}, nil
	}
	return ids, nil
}
