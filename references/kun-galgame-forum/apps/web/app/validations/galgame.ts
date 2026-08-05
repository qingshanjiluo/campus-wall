import { z } from 'zod'
import {
  KUN_RESOURCE_TYPE_CONST,
  KUN_RESOURCE_LANGUAGE_CONST,
  KUN_RESOURCE_PLATFORM_CONST
} from '~/constants/galgame'
import { PROVIDER_KEY_OPTIONS } from '~/constants/galgameResource'

const SORT_ORDER_CONST = ['asc', 'desc'] as const

// The wiki owns galgame `original_language` (30+ codes and growing: ja-jp /
// en-us / ru / zh-cn / ko-kr / es / uk / fr / …, both `xx` and `xx-yy`). The
// forum must accept any code the wiki already stored, or re-editing such a
// draft fails validation here and forces a manual re-pick. So validate the
// SHAPE (a BCP-47-ish short code) plus the legacy 'others' sentinel, rather
// than a hard enum of only the few we render names for.
const originalLanguageCode = z
  .string()
  .refine((v) => v === 'others' || /^[a-z]{2}(-[a-z]{2})?$/i.test(v), {
    message: '无效的游戏原语言代码'
  })

/*
 * Galgame
 */

const ProviderEnum = z.enum(PROVIDER_KEY_OPTIONS)

// helper: coerce query param (string | string[] | undefined) -> ProviderKey[]
const providerQueryArray = z.preprocess((v) => {
  if (Array.isArray(v)) {
    return v
  }
  if (typeof v === 'string') {
    if (!v) return []
    return v.split(',')
  }
  return []
}, z.array(ProviderEnum).default([]))

export const getGalgameSchema = z.object({
  page: z.coerce.number<number>().min(1).max(9999999),
  limit: z.coerce.number<number>().min(1).max(24),
  type: z.enum([...KUN_RESOURCE_TYPE_CONST, 'all']),
  language: z.enum([...KUN_RESOURCE_LANGUAGE_CONST, 'all']),
  platform: z.enum([...KUN_RESOURCE_PLATFORM_CONST, 'all']),
  sort_field: z.enum([
    'time',
    'created',
    'view',
    'view_1d',
    'view_7d',
    'view_30d',
    'release_date',
    'rating'
  ]),
  sort_order: z.enum(SORT_ORDER_CONST),
  include_providers: providerQueryArray,
  exclude_only_providers: providerQueryArray
})

export const getGalgameDetailSchema = z.object({
  galgame_id: z.coerce.number<number>().min(1).max(9999999)
})

export const getGalgameDuplicateSchema = z.object({
  vndb_id: z
    .string()
    .refine((s) => VNDBPattern.test(s), { message: '非法的 VNDB ID 格式' })
})

// Wire-format schema for the admin-direct create endpoint (POST /galgame).
// After the submission flow landed, this endpoint is gated to
// admin/moderator on both wiki and kungal — regular users go through
// submitGalgameSchema / POST /galgame/submit instead.
//
// Field names are snake_case to match the wiki API. The Vue store keeps
// camelCase names for ergonomics; the rename happens at the call site
// right before submission.
export const createGalgameSchema = z
  .object({
    vndb_id: z
      .string()
      .min(2)
      .max(10)
      .refine((value) => VNDBPattern.test(value), {
        message: '非法的 VNDB ID 格式'
      }),
    name_en_us: z
      .string()
      .max(100007, { message: '游戏名称最多 233 字' })
      .default(''),
    name_ja_jp: z
      .string()
      .max(100007, { message: '游戏名称最多 233 字' })
      .default(''),
    name_zh_cn: z
      .string()
      .max(100007, { message: '游戏名称最多 233 字' })
      .default(''),
    name_zh_tw: z
      .string()
      .max(100007, { message: '游戏名称最多 233 字' })
      .default(''),
    intro_en_us: z
      .string()
      .max(100007, { message: '游戏介绍最多 100007 字' })
      .default(''),
    intro_ja_jp: z
      .string()
      .max(100007, { message: '游戏介绍最多 100007 字' })
      .default(''),
    intro_zh_cn: z
      .string()
      .max(100007, { message: '游戏介绍最多 100007 字' })
      .default(''),
    intro_zh_tw: z
      .string()
      .max(100007, { message: '游戏介绍最多 100007 字' })
      .default(''),
    content_limit: z.enum(['sfw', 'nsfw']),
    age_limit: z.enum(['all', 'r18']).default('all'),
    original_language: originalLanguageCode.default('ja-jp'),
    // U1: "" = unknown (cleared on update). TBA is independent of date.
    release_date: z
      .string()
      .refine((v) => v === '' || /^\d{4}-\d{2}-\d{2}$/.test(v), {
        message: '发售日期格式应为 YYYY-MM-DD 或留空'
      })
      .default(''),
    release_date_tba: z.boolean().default(false),
    // Create accepts comma-separated string (matches wiki POST /galgame).
    aliases: z.string().default(''),
    banner: z.unknown()
  })
  .superRefine((data, ctx) => {
    const aliasArray = data.aliases.split(',')
    const isAliasLengthValid = aliasArray.length < 30
    if (!isAliasLengthValid) {
      ctx.addIssue({
        code: 'custom',
        message: 'Galgame 最多有 30 个别名',
        path: ['aliases']
      })
    }
    const hasInvalidAlias = aliasArray.some((a) => a.length > 500)
    if (hasInvalidAlias) {
      ctx.addIssue({
        code: 'custom',
        message: '每个 Galgame 别名最多 500 个字符',
        path: ['aliases']
      })
    }

    const hasAtLeastOneName =
      data.name_en_us || data.name_ja_jp || data.name_zh_cn || data.name_zh_tw
    if (!hasAtLeastOneName) {
      ctx.addIssue({
        code: 'custom',
        message: '至少需要填写一个语言版本的游戏名称',
        path: ['name_zh_cn']
      })
    }

    const hasAtLeastOneIntro =
      data.intro_en_us ||
      data.intro_ja_jp ||
      data.intro_zh_cn ||
      data.intro_zh_tw
    if (!hasAtLeastOneIntro) {
      ctx.addIssue({
        code: 'custom',
        message: '至少需要填写一个语言版本的游戏介绍',
        path: ['intro_zh_cn']
      })
    }
  })

// Wire-format schema for the user-submission flow (POST /galgame/submit).
//
// No vndb_id: the wiki has fully synced VNDB, so any VNDB-listed work is
// already a claimable status=2 draft — submission is exclusively for
// VNDB-unlisted (original / doujin / indie) works. VNDB titles go through
// the wizard's claim flow instead. Wiki still accepts vndb_id optionally
// on the endpoint; we just never collect it from this form.
export const submitGalgameSchema = z
  .object({
    name_en_us: z
      .string()
      .max(100007, { message: '游戏名称最多 233 字' })
      .default(''),
    name_ja_jp: z
      .string()
      .max(100007, { message: '游戏名称最多 233 字' })
      .default(''),
    name_zh_cn: z
      .string()
      .max(100007, { message: '游戏名称最多 233 字' })
      .default(''),
    name_zh_tw: z
      .string()
      .max(100007, { message: '游戏名称最多 233 字' })
      .default(''),
    intro_en_us: z
      .string()
      .max(100007, { message: '游戏介绍最多 100007 字' })
      .default(''),
    intro_ja_jp: z
      .string()
      .max(100007, { message: '游戏介绍最多 100007 字' })
      .default(''),
    intro_zh_cn: z
      .string()
      .max(100007, { message: '游戏介绍最多 100007 字' })
      .default(''),
    intro_zh_tw: z
      .string()
      .max(100007, { message: '游戏介绍最多 100007 字' })
      .default(''),
    content_limit: z.enum(['sfw', 'nsfw']),
    age_limit: z.enum(['all', 'r18']).default('all'),
    original_language: originalLanguageCode.default('ja-jp'),
    // U1: see createGalgameSchema; same rule + default.
    release_date: z
      .string()
      .refine((v) => v === '' || /^\d{4}-\d{2}-\d{2}$/.test(v), {
        message: '发售日期格式应为 YYYY-MM-DD 或留空'
      })
      .default(''),
    release_date_tba: z.boolean().default(false),
    aliases: z.string().default(''),
    banner: z.unknown()
  })
  .superRefine((data, ctx) => {
    const aliasArray = data.aliases.split(',')
    if (aliasArray.length >= 30) {
      ctx.addIssue({
        code: 'custom',
        message: 'Galgame 最多有 30 个别名',
        path: ['aliases']
      })
    }
    if (aliasArray.some((a) => a.length > 500)) {
      ctx.addIssue({
        code: 'custom',
        message: '每个 Galgame 别名最多 500 个字符',
        path: ['aliases']
      })
    }

    const hasAtLeastOneName =
      data.name_en_us || data.name_ja_jp || data.name_zh_cn || data.name_zh_tw
    if (!hasAtLeastOneName) {
      ctx.addIssue({
        code: 'custom',
        message: '至少需要填写一个语言版本的游戏名称',
        path: ['name_zh_cn']
      })
    }

    const hasAtLeastOneIntro =
      data.intro_en_us ||
      data.intro_ja_jp ||
      data.intro_zh_cn ||
      data.intro_zh_tw
    if (!hasAtLeastOneIntro) {
      ctx.addIssue({
        code: 'custom',
        message: '至少需要填写一个语言版本的游戏介绍',
        path: ['intro_zh_cn']
      })
    }
  })

// Wire-format schema for editing one's own pending/declined draft
// (PATCH /galgame/:gid). Same field set as submit, all optional — wiki
// merges with existing values for any field the user didn't touch.
export const patchDraftSchema = z.object({
  vndb_id: z
    .string()
    .max(10)
    .refine((value) => value === '' || VNDBPattern.test(value), {
      message: '非法的 VNDB ID 格式'
    })
    .optional(),
  name_en_us: z.string().max(100007).optional(),
  name_ja_jp: z.string().max(100007).optional(),
  name_zh_cn: z.string().max(100007).optional(),
  name_zh_tw: z.string().max(100007).optional(),
  intro_en_us: z.string().max(100007).optional(),
  intro_ja_jp: z.string().max(100007).optional(),
  intro_zh_cn: z.string().max(100007).optional(),
  intro_zh_tw: z.string().max(100007).optional(),
  content_limit: z.enum(['sfw', 'nsfw']).optional(),
  age_limit: z.enum(['all', 'r18']).optional(),
  original_language: originalLanguageCode.optional(),
  // U1: optional on patch — nil = keep; "" = clear to unknown; concrete
  // "YYYY-MM-DD" = set.
  release_date: z
    .string()
    .refine((v) => v === '' || /^\d{4}-\d{2}-\d{2}$/.test(v), {
      message: '发售日期格式应为 YYYY-MM-DD 或留空'
    })
    .optional(),
  release_date_tba: z.boolean().optional(),
  aliases: z.string().optional(),
  is_minor: z.boolean().optional()
})

export const updateGalgameLikeSchema = z.object({
  galgame_id: z.coerce.number<number>().min(1).max(9999999)
})

export const updateGalgameFavoriteSchema = z.object({
  galgame_id: z.coerce.number<number>().min(1).max(9999999)
})

/*
 * Resource
 */

export const getGalgameResourceSchema = z.object({
  galgame_id: z.coerce.number<number>().min(1).max(9999999)
})

export const getGalgameResourceDetailSchema = z.object({
  galgame_resource_id: z.coerce.number<number>().min(1).max(9999999)
})

export const createGalgameResourceSchema = z.object({
  galgame_id: z.coerce.number<number>().min(1).max(9999999),
  type: z.enum(KUN_RESOURCE_TYPE_CONST),
  link: z
    .array(
      z
        .url({ message: '您的资源链接必须是 URL' })
        .min(1)
        .max(1007, { message: '您的每条资源链接最大 1007 个字符' })
    )
    .min(1, { message: '您至少需要有一条资源链接' })
    .max(20, { message: '您最多有 20 条资源链接' }),
  language: z.enum(KUN_RESOURCE_LANGUAGE_CONST),
  platform: z.enum(KUN_RESOURCE_PLATFORM_CONST),
  size: z.string().refine((s) => ResourceSizePattern.test(s), {
    message: '非法的资源大小, 资源大小应该包含 MB 或者 GB'
  }),
  code: z
    .string()
    .max(1007, { message: '资源的提取码不可超过 1007 个字符' })
    .optional()
    .default(''),
  password: z
    .string()
    .max(1007, { message: '资源的解压码不可超过 1007 个字符' })
    .optional()
    .default(''),
  note: z
    .string()
    .max(1007, { message: '资源的备注不可超过 1007 个字符' })
    .optional()
    .default('')
})

export const updateGalgameResourceSchema = createGalgameResourceSchema.merge(
  z.object({
    galgame_resource_id: z.coerce.number<number>().min(1).max(9999999)
  })
)

export const updateGalgameResourceLikeSchema = z.object({
  galgame_resource_id: z.coerce.number<number>().min(1).max(9999999)
})

export const updateGalgameResourceExpireSchema = z.object({
  galgame_resource_id: z.coerce.number<number>().min(1).max(9999999)
})

export const updateGalgameResourceValidSchema = z.object({
  galgame_resource_id: z.coerce.number<number>().min(1).max(9999999)
})

export const deleteGalgameResourceSchema = z.object({
  galgame_resource_id: z.coerce.number<number>().min(1).max(9999999)
})

export const getGalgameLinkSchema = z.object({
  galgame_id: z.coerce.number<number>().min(1).max(9999999)
})

export const createGalgameLinkSchema = z.object({
  galgame_id: z.coerce.number<number>().min(1).max(9999999),
  name: z.string().min(1).max(107, { message: '相关链接名最大 107 个字符' }),
  link: z.string().min(1).max(500, { message: '相关链接的链接最多 500 个字符' })
})

export const deleteGalgameLinkSchema = z.object({
  galgame_link_id: z.coerce.number<number>().min(1).max(9999999)
})

export const getGalgameSeriesSchema = z.object({
  galgame_id: z.coerce.number<number>().min(1).max(9999999)
})
