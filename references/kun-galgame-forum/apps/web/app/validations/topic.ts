import { z } from 'zod'
import { KUN_TOPIC_TITLE_LENGTH_LIMIT } from '~/config/limit'
import {
  KUN_TOPIC_CATEGORY_CONST,
  KUN_TOPIC_SECTION_CONST,
  TOPIC_SORT_FIELD_CONST
} from '~/constants/topic'

const SORT_ORDER_CONST = ['asc', 'desc'] as const

/*
 * Topic
 */

export const getTopicSchema = z.object({
  page: z.coerce.number<number>().min(1).max(9999999),
  limit: z.coerce.number<number>().min(1).max(30),
  sort_field: z.enum(TOPIC_SORT_FIELD_CONST),
  sort_order: z.enum(SORT_ORDER_CONST),
  category: z.enum(KUN_TOPIC_CATEGORY_CONST)
})

export const getTopicDetailSchema = z.object({
  topic_id: z.coerce.number<number>().min(1).max(9999999)
})

export const createTopicSchema = z.object({
  title: z
    .string()
    .min(1, { message: '话题标题最少 1 个字符' })
    .max(KUN_TOPIC_TITLE_LENGTH_LIMIT, {
      message: `话题标题最大长度为 ${KUN_TOPIC_TITLE_LENGTH_LIMIT} 个字符`
    })
    .refine((t) => t.trim().length, { message: '话题标题最少为 1 个字符' }),
  content: z
    .string()
    .min(1, { message: '话题内容最少 1 个字符' })
    .max(100007, { message: '话题标题最大长度为 100007 个字符' })
    .refine((t) => t.trim().length, { message: '话题内容最少为 1 个字符' }),
  category: z.enum(KUN_TOPIC_CATEGORY_CONST),
  section: z
    .array(z.enum(KUN_TOPIC_SECTION_CONST))
    .min(1, { message: '您至少选择一个话题的分区' })
    .max(3, { message: '您至多选择三个话题的分区' }),
  is_nsfw: z.coerce.boolean({ message: '未找到话题的 NSFW 设置' }),
  // Optional 1..9 cover images, each a /image/<64hex> content token returned by
  // the /image/topic upload endpoint (see CoverPicker.vue). Empty = no covers.
  cover_images: z
    .array(
      z
        .string()
        .regex(/^\/image\/[0-9a-f]{64}$/, { message: '封面图格式不正确' })
    )
    .max(9, { message: '封面图最多 9 张' })
    .optional()
})

export const updateTopicSchema = createTopicSchema.extend({
  topic_id: z.coerce.number<number>().min(1).max(9999999)
})

export const updateTopicLikeSchema = z.object({
  topic_id: z.coerce.number<number>().min(1).max(9999999)
})

export const updateTopicDislikeSchema = z.object({
  topic_id: z.coerce.number<number>().min(1).max(9999999)
})

export const updateTopicUpvoteSchema = z.object({
  topic_id: z.coerce.number<number>().min(1).max(9999999)
})

export const updateTopicFavoriteSchema = z.object({
  topic_id: z.coerce.number<number>().min(1).max(9999999)
})

export const updateTopicBestAnswerSchema = z.object({
  topic_id: z.coerce.number<number>().min(1).max(9999999),
  reply_id: z.coerce.number<number>().min(1).max(9999999)
})

export const updateTopicHideStatusSchema = z.object({
  topic_id: z.coerce.number<number>().min(1).max(9999999)
})

/*
 * Reply
 */

export const getReplySchema = z.object({
  topic_id: z.coerce.number<number>().min(1).max(9999999),
  page: z.coerce.number<number>().min(1).max(9999999),
  limit: z.coerce.number<number>().min(1).max(30),
  sort_order: z.enum(SORT_ORDER_CONST)
})

export const getReplyDetailSchema = z.object({
  reply_id: z.coerce.number<number>().min(1).max(9999999)
})

// A reply is a single body now (multi-target tabs retired); @mention / #quote
// references live inline in `content` as tokens.
export const createReplySchema = z.object({
  topic_id: z.coerce.number<number>().min(1).max(9999999),
  content: z
    .string()
    .trim()
    .min(1, { message: '回复内容不能为空' })
    .max(10007, { message: '单条回复的最大长度为 10007 个字符' })
})

export const updateReplySchema = z.object({
  reply_id: z.coerce.number<number>().min(1).max(9999999),
  content: z
    .string()
    .trim()
    .min(1, { message: '回复内容不能为空' })
    .max(10007, { message: '单条回复的最大长度为 10007 个字符' })
})

export const updateReplyPinSchema = z.object({
  topic_id: z.coerce.number<number>().min(1).max(9999999),
  reply_id: z.coerce.number<number>().min(1).max(9999999)
})

export const updateReplyLikeSchema = z.object({
  reply_id: z.coerce.number<number>().min(1).max(9999999)
})

export const updateReplyDislikeSchema = z.object({
  reply_id: z.coerce.number<number>().min(1).max(9999999)
})

export const deleteReplySchema = z.object({
  reply_id: z.coerce.number<number>().min(1).max(9999999)
})

/*
 * Comment
 */

export const createCommentSchema = z.object({
  topic_id: z.coerce.number<number>().min(1).max(9999999),
  reply_id: z.coerce.number<number>().min(1).max(9999999),
  target_user_id: z.coerce.number<number>().min(1).max(9999999),
  content: z
    .string()
    .min(1, { message: '评论最少 1 个字符' })
    .max(1007, { message: '评论的最大长度为 1007 个字符' })
})

export const updateCommentLikeSchema = z.object({
  comment_id: z.coerce.number<number>().min(1).max(9999999)
})

export const deleteCommentSchema = z.object({
  comment_id: z.coerce.number<number>().min(1).max(9999999)
})
