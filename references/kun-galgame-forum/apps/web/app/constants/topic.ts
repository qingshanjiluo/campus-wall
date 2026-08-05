import type { KunSelectOption } from '@kungal/ui-vue'

export const KUN_TOPIC_CATEGORY: Record<string, string> = {
  galgame: 'Galgame',
  technique: '技术交流',
  others: '其它话题'
}

export const KUN_TOPIC_CATEGORY_CONST = [
  'galgame',
  'technique',
  'others',
  'all'
] as const

export const KUN_TOPIC_SECTION: Record<string, string> = {
  'g-walkthrough': '攻略',
  'g-chatting': '闲聊',
  'g-article': '文章',
  'g-seeking': '寻求资源',
  'g-news': '资讯',
  'g-releases': '新作消息',
  'g-other': '其它',
  't-crack': '逆向工程',
  't-web': 'Web',
  't-languages': '编程语言',
  't-help': '请求帮助',
  't-linux': 'Linux',
  't-practical': '实用技术',
  't-ai': 'AI',
  't-android': 'Android',
  't-adobe': 'Adobe',
  't-algorithm': '算法',
  't-other': '其它',
  'o-anime': '动漫',
  'o-comics': '漫画',
  'o-music': '音乐',
  'o-novel': '轻小说',
  'o-daily': '日常',
  'o-essay': '个人随笔',
  'o-forum': '论坛相关',
  'o-patch': '补丁网站',
  'o-other': '其它'
}

export const KUN_TOPIC_SECTION_CONST = [
  'g-walkthrough',
  'g-chatting',
  'g-article',
  'g-seeking',
  'g-news',
  'g-releases',
  'g-other',
  't-crack',
  't-web',
  't-languages',
  't-help',
  't-linux',
  't-practical',
  't-ai',
  't-android',
  't-adobe',
  't-algorithm',
  't-other',
  'o-anime',
  'o-comics',
  'o-music',
  'o-novel',
  'o-daily',
  'o-essay',
  'o-forum',
  'o-patch',
  'o-other'
] as const

export const KUN_TOPIC_PAGE_SORT_FIELD: Record<string, string> = {
  views: '浏览',
  created: '时间',
  all: '全部分类'
}

export const KUN_TOPIC_DETAIL_STATUS: Record<string, string> = {
  featured: '被推',
  normal: '正常',
  banned: '封禁',
  pinned: '置顶',
  essential: '精华'
}

export const KUN_TOPIC_REPLY_PANEL_POSITION_MAP: Record<string, string> = {
  master: '狗修金',
  reply: '杂鱼~♡'
}

export const topicSortFieldOptions = [
  { value: 'status_update_time', label: '更新时间' },
  { value: 'created', label: '创建时间' },
  { value: 'view', label: '浏览量' }
] as const

// /topic list sort fields (rendered as a KunSelect). The four view metrics
// (日/周/月/总浏览数) go at the end.
export type TopicListSortField =
  | 'status_update_time'
  | 'created'
  | 'view_1d'
  | 'view_7d'
  | 'view_30d'
  | 'view'

export const topicListSortFieldOptions: KunSelectOption<TopicListSortField>[] = [
  { value: 'status_update_time', label: '更新时间' },
  { value: 'created', label: '创建时间' },
  { value: 'view_1d', label: '日浏览数' },
  { value: 'view_7d', label: '周浏览数' },
  { value: 'view_30d', label: '月浏览数' },
  { value: 'view', label: '总浏览数' }
]

export const TOPIC_SORT_FIELD_CONST = [
  'created',
  'view',
  'view_1d',
  'view_7d',
  'view_30d',
  'status_update_time',
  'like',
  'favorite',
  'upvote'
] as const

export const topicSortCategoryOptions = [
  { value: 'all', label: '全部类型' },
  { value: 'galgame', label: 'Galgame' },
  { value: 'technique', label: '技术交流' },
  { value: 'others', label: '其它' }
] as const

export const TOPIC_CATEGORIES = {
  galgame: {
    key: 'galgame',
    label: 'Galgame',
    icon: 'lucide:gamepad-2'
  },
  technique: {
    key: 'technique',
    label: '技术交流',
    icon: 'lucide:drafting-compass'
  },
  others: {
    key: 'others',
    label: '其它话题',
    icon: 'lucide:circle-ellipsis'
  }
} as const

export type TopicCategoryKey = keyof typeof TOPIC_CATEGORIES

export const TOPIC_SECTIONS: Record<
  TopicCategoryKey,
  Record<string, string>
> = {
  galgame: {
    'g-walkthrough': '攻略',
    'g-chatting': '闲聊',
    'g-article': '文章',
    'g-seeking': '寻求资源',
    'g-news': '资讯',
    'g-releases': '新作消息',
    'g-other': '其它'
  },
  technique: {
    't-crack': '逆向工程',
    't-web': 'Web',
    't-languages': '编程语言',
    't-help': '请求帮助',
    't-linux': 'Linux',
    't-practical': '实用技术',
    't-ai': 'AI',
    't-android': 'Android',
    't-adobe': 'Adobe',
    't-algorithm': '算法',
    't-other': '其它'
  },
  others: {
    'o-anime': '动漫',
    'o-comics': '漫画',
    'o-music': '音乐',
    'o-novel': '轻小说',
    'o-daily': '日常',
    'o-essay': '个人随笔',
    'o-forum': '论坛相关',
    'o-patch': '补丁网站',
    'o-other': '其它'
  }
}

export const TOPIC_POLL_VISIBILITY_OPTIONS = [
  { value: 'always', label: '任何人可见结果' },
  { value: 'after_vote', label: '投票后可见结果' },
  { value: 'after_deadline', label: '结束后可见结果' }
] as const

export const TOPIC_POLL_VISIBILITY_MAP: Record<string, string> = {
  always: '任何人可见结果',
  after_vote: '投票后可见结果',
  after_deadline: '结束后可见结果'
}
