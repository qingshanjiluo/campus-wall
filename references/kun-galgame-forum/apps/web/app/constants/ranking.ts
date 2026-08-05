import type { KunTabItem } from '@kungal/ui-vue'

export type RankingTopicSortField =
  | 'view'
  | 'upvote'
  | 'like'
  | 'favorite'
  | 'reply'
  | 'comment'

export type RankingGalgameSortField =
  | 'view'
  | 'like'
  | 'favorite'
  | 'resource'
  | 'rating'

export type RankingUserSortField =
  | 'moemoepoint'
  | 'topic'
  | 'reply_created'
  | 'comment_created'
  | 'galgame_resource'

export interface RankingItem {
  index: number
  icon: string
  name: string
  label: string
}

export interface RankingTopic extends RankingItem {
  sortField: RankingTopicSortField
}

export interface RankingGalgame extends RankingItem {
  sortField: RankingGalgameSortField
}

export interface RankingUser extends RankingItem {
  sortField: RankingUserSortField
}

export const topicSortItem: RankingTopic[] = [
  {
    index: 1,
    icon: 'lucide:eye',
    name: 'view',
    sortField: 'view',
    label: '浏览数'
  },
  {
    index: 2,
    icon: 'carbon:reply',
    name: 'reply',
    sortField: 'reply',
    label: '回复数'
  },
  {
    index: 3,
    icon: 'uil:comment-dots',
    name: 'comment',
    sortField: 'comment',
    label: '评论数'
  },
  {
    index: 4,
    icon: 'lucide:thumbs-up',
    name: 'like',
    sortField: 'like',
    label: '点赞数'
  },
  {
    index: 5,
    icon: 'lucide:sparkles',
    name: 'upvote',
    sortField: 'upvote',
    label: '被推数'
  },
  {
    index: 6,
    icon: 'lucide:heart',
    name: 'favorite',
    sortField: 'favorite',
    label: '收藏数'
  }
]

export const galgameSortItem: RankingGalgame[] = [
  {
    index: 1,
    icon: 'lucide:eye',
    name: 'view',
    sortField: 'view',
    label: '浏览数'
  },
  {
    index: 2,
    icon: 'lucide:thumbs-up',
    name: 'like',
    sortField: 'like',
    label: '点赞数'
  },
  {
    index: 3,
    icon: 'lucide:heart',
    name: 'favorite',
    sortField: 'favorite',
    label: '收藏数'
  },
  {
    index: 4,
    icon: 'lucide:box',
    name: 'resource',
    sortField: 'resource',
    label: '资源数'
  },
  {
    index: 5,
    icon: 'lucide:star',
    name: 'rating',
    sortField: 'rating',
    label: '评分'
  }
]

export const userSortItem: RankingUser[] = [
  {
    index: 1,
    icon: 'lucide:lollipop',
    name: 'moemoepoint',
    sortField: 'moemoepoint',
    label: '萌萌点'
  },
  // TODO:
  // {
  //   index: 2,
  //   icon: 'lucide:sparkles',
  //   name: 'upvote',
  //   sortField: 'upvote'
  // },
  // {
  //   index: 3,
  //   icon: 'lucide:thumbs-up',
  //   name: 'like',
  //   sortField: 'like'
  // },
  {
    index: 2,
    icon: 'lucide:square-gantt-chart',
    name: 'topic',
    sortField: 'topic',
    label: '话题数'
  },
  {
    index: 3,
    icon: 'carbon:reply',
    name: 'reply_created',
    sortField: 'reply_created',
    label: '回复数'
  },
  {
    index: 4,
    icon: 'uil:comment-dots',
    name: 'comment_created',
    sortField: 'comment_created',
    label: '评论数'
  },
  {
    index: 5,
    icon: 'lucide:box',
    name: 'galgame_resource',
    sortField: 'galgame_resource',
    label: 'Galgame 资源'
  }
  // NOTE: a "Galgame 数" (galgames added by a user) sort was removed — post
  // wiki-migration the forum's local galgame table has no creator column, so
  // there's no local source to rank by. Restoring it needs wiki (infra) data.
]

export const rankingPageTabs: KunTabItem[] = [
  {
    textValue: '话题',
    value: 'topic',
    href: '/ranking/topic'
  },
  {
    textValue: 'Galgame',
    value: 'galgame',
    href: '/ranking/galgame'
  },
  {
    textValue: '用户',
    value: 'user',
    href: '/ranking/user'
  }
]

export const rankingPageMetaData: Record<
  string,
  {
    title: string
    description: string
  }
> = {
  topic: {
    title: '话题排行',
    description:
      '查看关于 Galgame 话题的浏览数, 点赞数, 回复数, 评论数排行, 浏览最多人关注的 Galgame 话题'
  },
  galgame: {
    title: 'Galgame 排行',
    description:
      '最强大的 Galgame 排行, 根据本站所有用户的历史数据综合得出, 具有良好的参考价值。适合用于 Galgame, Galgame 资源, Galgame 交流'
  },
  user: {
    title: '用户排行',
    description:
      '用户排行, 查看最强大的 Galgamer, 查看发布 Galgame 资源最多的用户, 查看 Galgamer 相关的动态, Galgame, Galgame 资源与话题'
  }
}
