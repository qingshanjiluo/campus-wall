import { z } from 'zod'

export const KUN_COLLECTION_VISIBILITY_CONST = [
  'public',
  'private',
  'restricted'
] as const

// Create / edit a galgame collection (收藏夹). Mirrors the Go DTO
// CreateCollectionRequest / UpdateCollectionRequest (snake_case wire fields).
export const createCollectionSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, { message: '收藏夹名称不能为空' })
    .max(60, { message: '收藏夹名称不能超过 60 个字符' }),
  description: z
    .string()
    .max(500, { message: '收藏夹描述不能超过 500 个字符' })
    .default(''),
  visibility: z.enum(KUN_COLLECTION_VISIBILITY_CONST),
  viewer_ids: z
    .array(z.number().int().positive())
    .max(100, { message: '指定可见用户不能超过 100 人' })
    .default([])
})

export type CreateCollectionPayload = z.infer<typeof createCollectionSchema>

// Full membership of a galgame across the user's collections.
export const setCollectionMembershipSchema = z.object({
  collection_ids: z
    .array(z.number().int().positive())
    .max(50, { message: '一个游戏最多同时收藏到 50 个收藏夹' })
})
