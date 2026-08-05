import { z } from 'zod'

// 233 is the BE article slug cap (`max=233` on Doc article DTO). Category
// and Doc tag entities cap their slugs at 100 (BE DocCategory/DocTag DTOs)
// — those schemas override `slug` with a stricter rule below.
const slugSchema = z
  .string()
  .trim()
  .min(1, 'slug 不能为空')
  .max(233, 'slug 最长 233 个字符')
  .regex(/^[a-z0-9-]+$/i, 'slug 仅能包含字母、数字与连接符')
  .transform((value) => value.toLowerCase())

// Category and Tag slugs are capped at 100 on the BE side.
const shortSlugSchema = z
  .string()
  .trim()
  .min(1, 'slug 不能为空')
  .max(100, 'slug 最长 100 个字符')
  .regex(/^[a-z0-9-]+$/i, 'slug 仅能包含字母、数字与连接符')
  .transform((value) => value.toLowerCase())

const optionalString = (max: number, defaultValue = '') =>
  z.string().trim().max(max).optional().default(defaultValue)

const paginationSchema = {
  page: z.coerce.number<number>().min(1).max(9999999).default(1),
  limit: z.coerce.number<number>().min(1).max(100).default(20),
  keyword: z.string().trim().max(200).optional().default('')
}

export const getDocCategoryListSchema = z.object(paginationSchema)

// All caps mirror apps/api/internal/doc/dto/category_dto.go exactly.
export const createDocCategorySchema = z.object({
  slug: shortSlugSchema,
  title: z
    .string()
    .trim()
    .min(1, '分类标题不能为空')
    .max(233, '分类标题最长 233 个字符'),
  description: optionalString(500),
  icon: optionalString(200),
  sort_order: z.coerce.number<number>().int().min(0).max(9999).default(0)
})

export const updateDocCategorySchema = createDocCategorySchema.merge(
  z.object({
    category_id: z.coerce.number<number>().min(1).max(9999999)
  })
)

export const deleteDocCategorySchema = z.object({
  category_id: z.coerce.number<number>().min(1).max(9999999)
})

export const getDocTagListSchema = z.object(paginationSchema)

// All caps mirror apps/api/internal/doc/dto/tag_dto.go exactly.
export const createDocTagSchema = z.object({
  slug: shortSlugSchema,
  title: z
    .string()
    .trim()
    .min(1, '标签名称不能为空')
    .max(100, '标签名称最长 100 个字符'),
  description: optionalString(500)
})

export const updateDocTagSchema = createDocTagSchema.merge(
  z.object({
    tag_id: z.coerce.number<number>().min(1).max(9999999)
  })
)

export const deleteDocTagSchema = z.object({
  tag_id: z.coerce.number<number>().min(1).max(9999999)
})

const docArticleOrderFields = [
  'publishedTime',
  'created',
  'view',
  'updated'
] as const

export const getDocArticleListSchema = z.object({
  ...paginationSchema,
  category_id: z.coerce.number<number>().min(1).max(9999999).optional(),
  status: z.coerce.number<number>().int().min(0).max(2).optional(),
  is_pin: z.coerce.boolean().optional(),
  tag_id: z.coerce.number<number>().min(1).max(9999999).optional(),
  order_by: z.enum(docArticleOrderFields).default('publishedTime'),
  sort_order: z.enum(['asc', 'desc']).default('desc')
})

// All caps mirror apps/api/internal/doc/dto/article_dto.go.
// `description` and `banner` previously diverged from BE — description
// was capped at 777 (BE 1000) and banner at 777 (BE 500), with FE
// description.min=1 BE has no min. Aligned both ways here.
const docArticleBaseSchema = z.object({
  title: z
    .string()
    .trim()
    .min(1, '文档标题不能为空')
    .max(233, '标题最长 233 个字符'),
  slug: slugSchema,
  description: optionalString(1000),
  banner: optionalString(500),
  status: z.coerce.number<number>().int().min(0).max(2).default(1),
  is_pin: z.coerce.boolean().default(false),
  content_markdown: z
    .string()
    .trim()
    .min(1, '正文内容不能为空')
    .max(100007, '正文长度超出限制'),
  category_id: z.coerce.number<number>().min(1).max(9999999),
  tag_ids: z
    .array(z.coerce.number<number>().min(1).max(9999999))
    .optional()
    .default([])
})

export const createDocArticleSchema = docArticleBaseSchema

export const updateDocArticleSchema = docArticleBaseSchema.merge(
  z.object({
    article_id: z.coerce.number<number>().min(1).max(9999999)
  })
)

export const deleteDocArticleSchema = z.object({
  article_id: z.coerce.number<number>().min(1).max(9999999)
})
