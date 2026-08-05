export const KUN_ACTIVITY_TYPE_TYPE: Record<string, string> = {
  GALGAME_CREATION: 'Galgame',
  GALGAME_RATING_CREATION: 'Galgame 评分',
  GALGAME_RATING_COMMENT_CREATION: 'Galgame 评分评论',
  TOPIC_CREATION: '新话题',
  TOPIC_UPVOTE: '话题被推',
  MESSAGE_UPVOTE: '话题被推',
  MESSAGE_SOLUTION: '最佳答案',
  TOPIC_REPLY_CREATION: '话题回复',
  TOPIC_COMMENT_CREATION: '话题评论',
  GALGAME_WEBSITE_CREATION: 'Galgame 网站',
  GALGAME_RESOURCE_CREATION: 'Galgame 资源',
  GALGAME_QUIZ_CREATION: 'Galgame 题目',
  GALGAME_EDIT: 'Galgame 编辑',
  GALGAME_PR_CREATION: '提出更新请求',
  GALGAME_COMMENT_CREATION: 'Galgame 评论',
  GALGAME_WEBSITE_COMMENT_CREATION: 'Galgame 网站评论',
  TODO_CREATION: '待办',
  UPDATE_LOG_CREATION: '更新日志',
  TOOLSET_CREATION: 'Galgame 工具',
  TOOLSET_RESOURCE_CREATION: '工具资源',
  TOOLSET_COMMENT_CREATION: '工具评论',
  GALGAME_RESOURCE_COMMENT_CREATION: 'Galgame 资源评论',
  GALGAME_QUIZ_COMMENT_CREATION: 'Galgame 题目讨论'
}

export const KUN_ALLOWED_ACTIVITY_TYPE = [
  'GALGAME_CREATION',
  'GALGAME_COMMENT_CREATION',
  'GALGAME_RATING_CREATION',
  'GALGAME_RATING_COMMENT_CREATION',
  'GALGAME_WEBSITE_CREATION',
  'GALGAME_WEBSITE_COMMENT_CREATION',
  'GALGAME_RESOURCE_CREATION',
  'GALGAME_RESOURCE_COMMENT_CREATION',
  'GALGAME_QUIZ_CREATION',
  'GALGAME_QUIZ_COMMENT_CREATION',
  'GALGAME_EDIT',
  'GALGAME_PR_CREATION',
  'TOOLSET_CREATION',
  'TOOLSET_RESOURCE_CREATION',
  'TOOLSET_COMMENT_CREATION',
  'TOPIC_CREATION',
  'TOPIC_REPLY_CREATION',
  'TOPIC_COMMENT_CREATION',
  'TOPIC_UPVOTE',
  'TODO_CREATION',
  'UPDATE_LOG_CREATION',
  'MESSAGE_UPVOTE',
  'MESSAGE_SOLUTION'
] as const

// Grouped view of the activity types for the /activity/category picker. A flat
// 18-item tab row was unscannable and overflowed, so the picker is a grouped
// dropdown instead. INVARIANT: every key in KUN_ACTIVITY_TYPE_TYPE must appear
// in exactly one group's `types` (ordering within a group is the display order).
export const KUN_ACTIVITY_GROUPS: { label: string; types: string[] }[] = [
  {
    label: 'Galgame',
    types: [
      'GALGAME_CREATION',
      'GALGAME_EDIT',
      'GALGAME_PR_CREATION',
      'GALGAME_RESOURCE_CREATION',
      'GALGAME_RESOURCE_COMMENT_CREATION',
      'GALGAME_QUIZ_CREATION',
      'GALGAME_QUIZ_COMMENT_CREATION',
      'GALGAME_RATING_CREATION',
      'GALGAME_RATING_COMMENT_CREATION',
      'GALGAME_COMMENT_CREATION',
      'GALGAME_WEBSITE_CREATION',
      'GALGAME_WEBSITE_COMMENT_CREATION'
    ]
  },
  {
    label: '社区',
    types: [
      'TOPIC_CREATION',
      'TOPIC_REPLY_CREATION',
      'TOPIC_COMMENT_CREATION',
      'TOPIC_UPVOTE',
      'MESSAGE_UPVOTE',
      'MESSAGE_SOLUTION'
    ]
  },
  {
    label: '工具集',
    types: [
      'TOOLSET_CREATION',
      'TOOLSET_RESOURCE_CREATION',
      'TOOLSET_COMMENT_CREATION'
    ]
  },
  {
    label: '站务',
    types: ['TODO_CREATION', 'UPDATE_LOG_CREATION']
  }
]

export const KUN_ACTIVITY_ICON_MAP: Record<string, string> = {
  GALGAME_CREATION: 'lucide:gamepad-2',
  GALGAME_RATING_CREATION: 'lucide:star',
  GALGAME_RATING_COMMENT_CREATION: 'lucide:message-square-text',
  GALGAME_COMMENT_CREATION: 'lucide:message-square',
  GALGAME_WEBSITE_CREATION: 'lucide:globe',
  GALGAME_WEBSITE_COMMENT_CREATION: 'lucide:message-square-text',
  GALGAME_RESOURCE_CREATION: 'lucide:box',
  GALGAME_QUIZ_CREATION: 'lucide:brain',
  GALGAME_RESOURCE_COMMENT_CREATION: 'lucide:message-square-text',
  GALGAME_QUIZ_COMMENT_CREATION: 'lucide:message-square-text',
  GALGAME_EDIT: 'lucide:file-pen-line',
  GALGAME_PR_CREATION: 'lucide:git-pull-request',
  TOOLSET_CREATION: 'lucide:wrench',
  TOOLSET_RESOURCE_CREATION: 'lucide:package-plus',
  TOOLSET_COMMENT_CREATION: 'lucide:wrench',
  TOPIC_CREATION: 'icon-park-outline:topic',
  TOPIC_UPVOTE: 'lucide:trending-up',
  TOPIC_REPLY_CREATION: 'carbon:reply',
  TOPIC_COMMENT_CREATION: 'lucide:message-circle-more',
  TODO_CREATION: 'lucide:list-checks',
  UPDATE_LOG_CREATION: 'lucide:file-clock',
  MESSAGE_UPVOTE: 'lucide:sparkles',
  MESSAGE_SOLUTION: 'lucide:bookmark-check'
}

// ── Configurable home-feed tabs (设置 → 动态) ──────────────────────────────────
// A custom tab is a name + icon + a set of "kinds". Kinds are activity types,
// EXCEPT topics are split into 普通话题 (TOPIC_NORMAL) / 资源求助话题
// (TOPIC_RESOURCE_HELP) — backend pseudo-types (service.resolveKinds) that
// collapse to TOPIC_CREATION + a section filter, so 资源/求助 topics can be routed
// independently (the 话题 tab takes 普通话题, the 资源和求助话题 tab takes 资源求助话题).
export interface KunFeedKind {
  value: string
  label: string
  icon: string
}

export const KUN_FEED_KIND_GROUPS: { label: string; kinds: KunFeedKind[] }[] = [
  {
    label: '话题',
    kinds: [
      { value: 'TOPIC_NORMAL', label: '话题', icon: 'icon-park-outline:topic' },
      {
        value: 'TOPIC_RESOURCE_HELP',
        label: '资源/求助话题',
        icon: 'lucide:life-buoy'
      },
      {
        value: 'TOPIC_REPLY_CREATION',
        label: '话题回复',
        icon: 'carbon:reply'
      },
      {
        value: 'TOPIC_COMMENT_CREATION',
        label: '话题评论',
        icon: 'lucide:message-circle-more'
      },
      { value: 'TOPIC_UPVOTE', label: '推话题', icon: 'lucide:trending-up' },
      {
        value: 'MESSAGE_SOLUTION',
        label: '最佳答案',
        icon: 'lucide:bookmark-check'
      }
    ]
  },
  {
    label: 'Galgame',
    kinds: [
      { value: 'GALGAME_CREATION', label: '新游戏', icon: 'lucide:gamepad-2' },
      {
        value: 'GALGAME_EDIT',
        label: '游戏编辑',
        icon: 'lucide:file-pen-line'
      },
      {
        value: 'GALGAME_PR_CREATION',
        label: '更新请求',
        icon: 'lucide:git-pull-request'
      },
      {
        value: 'GALGAME_COMMENT_CREATION',
        label: '游戏评论',
        icon: 'lucide:message-square'
      },
      {
        value: 'GALGAME_RATING_CREATION',
        label: '游戏评分',
        icon: 'lucide:star'
      },
      {
        value: 'GALGAME_RATING_COMMENT_CREATION',
        label: '评分评论',
        icon: 'lucide:message-square-text'
      },
      {
        value: 'GALGAME_RESOURCE_CREATION',
        label: '游戏资源',
        icon: 'lucide:box'
      },
      {
        value: 'GALGAME_QUIZ_CREATION',
        label: '游戏题目',
        icon: 'lucide:brain'
      },
      {
        value: 'GALGAME_WEBSITE_CREATION',
        label: '网站',
        icon: 'lucide:globe'
      },
      {
        value: 'GALGAME_WEBSITE_COMMENT_CREATION',
        label: '网站评论',
        icon: 'lucide:message-square-text'
      }
    ]
  },
  {
    label: '工具',
    kinds: [
      { value: 'TOOLSET_CREATION', label: '工具', icon: 'lucide:wrench' },
      {
        value: 'TOOLSET_RESOURCE_CREATION',
        label: '工具资源',
        icon: 'lucide:package-plus'
      },
      {
        value: 'TOOLSET_COMMENT_CREATION',
        label: '工具评论',
        icon: 'lucide:wrench'
      }
    ]
  },
  {
    label: '站务',
    kinds: [
      { value: 'TODO_CREATION', label: '待办', icon: 'lucide:list-checks' },
      {
        value: 'UPDATE_LOG_CREATION',
        label: '更新日志',
        icon: 'lucide:file-clock'
      }
    ]
  }
]

export interface KunFeedTab {
  id: string
  name: string
  icon: string
  kinds: string[]
}

// 全站动态 = topics + galgame 评分/网站, toolsets + notes. EXCLUDES 资源/求助话题,
// 游戏资源, 游戏评论, and the galgame 新游戏 / 游戏编辑 / 更新请求 kinds — those
// live in their own tabs (Galgame / 资源) and shouldn't flood the main stream.
const KUN_ALL_TAB_KINDS = [
  'TOPIC_NORMAL',
  'TOPIC_REPLY_CREATION',
  'TOPIC_COMMENT_CREATION',
  'TOPIC_UPVOTE',
  'MESSAGE_SOLUTION',
  'GALGAME_QUIZ_CREATION',
  'GALGAME_QUIZ_COMMENT_CREATION',
  'GALGAME_RESOURCE_COMMENT_CREATION',
  'GALGAME_RATING_CREATION',
  'GALGAME_RATING_COMMENT_CREATION',
  'GALGAME_WEBSITE_CREATION',
  'GALGAME_WEBSITE_COMMENT_CREATION',
  'TOOLSET_CREATION',
  'TOOLSET_RESOURCE_CREATION',
  'TOOLSET_COMMENT_CREATION',
  'TODO_CREATION',
  'UPDATE_LOG_CREATION'
]

// Bump when the DEFAULT tab structure changes (new / renamed / removed tabs) so
// the settings store can force a one-time reset of stale persisted tabs (see its
// afterHydrate). NOTE: this also resets users' CUSTOM tabs — acceptable here.
export const KUN_FEED_TABS_VERSION = 6

// Default tabs. Stable ids so the ?tab= URL + the active selection survive edits.
// The FIRST tab is the default landing tab.
//   话题        — normal topics only (TOPIC_NORMAL), rendered as the feed's rich
//                 topic card (this tab excludes replies/comments)
//   全站动态     — the full activity feed (KUN_ALL_TAB_KINDS)
//   Gal 资源    — galgame resources only (no topics)
//   资源和求助    — only the three 资源/求助 topic sections (TOPIC_RESOURCE_HELP)
export const KUN_DEFAULT_FEED_TABS: KunFeedTab[] = [
  {
    id: 'topic',
    name: '话题',
    icon: 'icon-park-outline:topic',
    kinds: ['TOPIC_NORMAL']
  },
  {
    id: 'galgame',
    name: 'Galgame',
    icon: 'lucide:gamepad-2',
    kinds: [
      'GALGAME_CREATION',
      'GALGAME_EDIT',
      'GALGAME_PR_CREATION',
      'GALGAME_COMMENT_CREATION',
      'GALGAME_QUIZ_CREATION',
      'GALGAME_QUIZ_COMMENT_CREATION',
      'GALGAME_RATING_CREATION',
      'GALGAME_RATING_COMMENT_CREATION',
      'GALGAME_WEBSITE_CREATION',
      'GALGAME_WEBSITE_COMMENT_CREATION',
      'TOOLSET_CREATION',
      'TOOLSET_RESOURCE_CREATION',
      'TOOLSET_COMMENT_CREATION'
    ]
  },
  {
    id: 'all',
    name: '全站动态',
    icon: 'lucide:layers',
    kinds: [...KUN_ALL_TAB_KINDS]
  },
  {
    id: 'resource',
    name: 'Gal 资源',
    icon: 'lucide:box',
    kinds: ['GALGAME_RESOURCE_CREATION', 'GALGAME_RESOURCE_COMMENT_CREATION']
  },
  {
    id: 'resource-help-topic',
    name: '资源和求助',
    icon: 'lucide:life-buoy',
    kinds: ['TOPIC_RESOURCE_HELP']
  },
  {
    id: 'others',
    name: '其他',
    icon: 'lucide:layout-grid',
    kinds: ['TODO_CREATION', 'UPDATE_LOG_CREATION']
  }
]
