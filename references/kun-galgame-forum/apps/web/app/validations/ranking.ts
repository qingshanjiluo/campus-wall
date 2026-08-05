import { z } from 'zod'

export const getUserRankingSchema = z.object({
  page: z.coerce.number<number>().min(1).max(9999999),
  limit: z.coerce.number<number>().min(1).max(100),
  sort_field: z.enum([
    'moemoepoint',
    'topic',
    'reply_created',
    'comment_created',
    'galgame_resource'
  ]),
  sort_order: z.enum(['asc', 'desc'])
})

export const getTopicRankingSchema = z.object({
  page: z.coerce.number<number>().min(1).max(9999999),
  limit: z.coerce.number<number>().min(1).max(100),
  sort_field: z.enum(['view', 'upvote', 'like', 'favorite', 'reply', 'comment']),
  sort_order: z.enum(['asc', 'desc'])
})

export const getGalgameRankingSchema = z.object({
  page: z.coerce.number<number>().min(1).max(9999999),
  limit: z.coerce.number<number>().min(1).max(100),
  sort_field: z.enum(['view', 'like', 'favorite', 'resource', 'rating']),
  sort_order: z.enum(['asc', 'desc'])
})
