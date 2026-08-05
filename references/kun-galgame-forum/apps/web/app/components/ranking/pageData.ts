import type {
  getTopicRankingSchema,
  getGalgameRankingSchema,
  getUserRankingSchema
} from '~/validations/ranking'
import type { z } from 'zod'

type Topic = z.infer<typeof getTopicRankingSchema>
type Galgame = z.infer<typeof getGalgameRankingSchema>
type User = z.infer<typeof getUserRankingSchema>

export const topicRankingPageData = reactive<Topic>({
  page: 1,
  limit: 50,
  sort_field: 'view',
  sort_order: 'desc'
})

export const galgameRankingPageData = reactive<Galgame>({
  page: 1,
  limit: 50,
  sort_field: 'view',
  sort_order: 'desc'
})

export const userRankingPageData = reactive<User>({
  page: 1,
  limit: 50,
  sort_field: 'moemoepoint',
  sort_order: 'desc'
})

export const getRankClasses = (index: number) => {
  if (index === 0) {
    return 'bg-warning-400/20 border-warning-500/50'
  }
  if (index === 1) {
    return 'bg-default-400/20 border-default-500/50'
  }
  if (index === 2) {
    return 'bg-info-400/20 border-info-500/50'
  }
  // Past the podium: an opaque surface (like the other content cards) instead of
  // a see-through row.
  return 'bg-content1'
}
