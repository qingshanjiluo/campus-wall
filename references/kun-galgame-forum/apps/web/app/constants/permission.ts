import type { ForumPermission } from '~/composables/useCan'

// Human-facing metadata for the permission-management admin page. Typed against
// ForumPermission so a change to the pure-forum vocabulary in useCan.ts breaks
// the build HERE (a missing / stray key is a compile error) — that coupling is
// deliberate: labels can never silently drift from the wire contract.

export interface KunPermissionMeta {
  label: string
  group: string
}

// Row grouping order for the matrix (top → bottom). Every ForumPermission's
// `group` below is one of these.
export const KUN_PERMISSION_GROUP_ORDER = [
  '话题',
  '回复',
  '评论',
  '投票',
  'Galgame',
  '题目',
  '资源',
  '评分',
  '工具集',
  '文档',
  '网站',
  '友链',
  '更新日志',
  '管理'
] as const

// The 43 pure-forum permissions → concise Chinese label + group. Insertion
// order follows the backend catalog's canonical order within each group.
export const KUN_PERMISSION_META: Record<ForumPermission, KunPermissionMeta> = {
  // 话题
  'topic.edit_any': { label: '编辑任意话题', group: '话题' },
  'topic.hide': { label: '隐藏话题', group: '话题' },
  'topic.set_best_answer': { label: '设置最佳答案', group: '话题' },
  // 回复
  'reply.delete_any': { label: '删除任意回复', group: '回复' },
  'reply.pin': { label: '置顶回复', group: '回复' },
  // 评论
  'comment.topic.delete': { label: '删除话题评论', group: '评论' },
  'comment.galgame.edit': { label: '编辑 Galgame 评论', group: '评论' },
  'comment.galgame.delete': { label: '删除 Galgame 评论', group: '评论' },
  'comment.rating.edit': { label: '编辑评分评论', group: '评论' },
  'comment.rating.delete': { label: '删除评分评论', group: '评论' },
  'comment.website.edit': { label: '编辑网站评论', group: '评论' },
  'comment.website.delete': { label: '删除网站评论', group: '评论' },
  'comment.toolset.edit': { label: '编辑工具集评论', group: '评论' },
  'comment.toolset.delete': { label: '删除工具集评论', group: '评论' },
  'comment.resource.edit': { label: '编辑资源评论', group: '评论' },
  'comment.resource.delete': { label: '删除资源评论', group: '评论' },
  'comment.quiz.edit': { label: '编辑题目评论', group: '评论' },
  'comment.quiz.delete': { label: '删除题目评论', group: '评论' },
  // 投票
  'poll.create_any': { label: '为任意话题创建投票', group: '投票' },
  'poll.edit_any': { label: '编辑任意投票', group: '投票' },
  'poll.delete_any': { label: '删除任意投票', group: '投票' },
  'poll.view_restricted': { label: '查看受限/匿名投票结果', group: '投票' },
  // Galgame
  'galgame.ban_resource_publish': {
    label: '禁止游戏资源发布',
    group: 'Galgame'
  },
  // 题目
  'quiz.edit_any': { label: '编辑任意题目', group: '题目' },
  'quiz.delete_any': { label: '删除任意题目', group: '题目' },
  // 资源
  'resource.edit_any': { label: '编辑任意游戏资源', group: '资源' },
  'resource.delete_any': { label: '删除任意游戏资源', group: '资源' },
  // 评分
  'rating.delete_any': { label: '删除任意评分', group: '评分' },
  // 工具集
  'toolset.edit_any': { label: '编辑任意工具集', group: '工具集' },
  'toolset.delete_any': { label: '删除任意工具集', group: '工具集' },
  'toolset.resource.edit_any': {
    label: '编辑任意工具集资源',
    group: '工具集'
  },
  'toolset.resource.delete_any': {
    label: '删除任意工具集资源',
    group: '工具集'
  },
  'toolset.upload_bypass': { label: '向任意工具集上传', group: '工具集' },
  // 文档
  'doc.create': { label: '创建文档', group: '文档' },
  'doc.edit': { label: '编辑文档', group: '文档' },
  'doc.delete': { label: '删除文档', group: '文档' },
  // 网站
  'website.create': { label: '创建网站', group: '网站' },
  'website.edit': { label: '编辑网站', group: '网站' },
  'website.delete': { label: '删除网站', group: '网站' },
  // 友链
  'friend_link.create': { label: '创建友链', group: '友链' },
  'friend_link.edit': { label: '编辑友链', group: '友链' },
  'friend_link.delete': { label: '删除友链', group: '友链' },
  // 更新日志
  'update_log.create': { label: '创建更新日志', group: '更新日志' },
  'update_log.edit': { label: '编辑更新日志', group: '更新日志' },
  'update_log.delete': { label: '删除更新日志', group: '更新日志' },
  // 管理
  'admin.dashboard': { label: '管理总览与统计', group: '管理' },
  'user.purge_content': { label: '清除用户全部内容', group: '管理' }
}

// The canonical 43-key list, in declaration order. Used to iterate the editable
// universe when computing deltas (never a no-op outside these keys).
export const KUN_PERMISSION_KEYS = Object.keys(
  KUN_PERMISSION_META
) as ForumPermission[]

// The 43 keys pre-bucketed by group in KUN_PERMISSION_GROUP_ORDER — the shared
// row layout for BOTH the role matrix and the per-user override panel. Static
// (the vocabulary is compile-time constant), so it's a plain array, not a
// computed. Empty groups are dropped.
export const KUN_PERMISSION_GROUPS: {
  group: string
  perms: ForumPermission[]
}[] = KUN_PERMISSION_GROUP_ORDER.map((group) => ({
  group,
  perms: KUN_PERMISSION_KEYS.filter(
    (key) => KUN_PERMISSION_META[key].group === group
  )
})).filter((entry) => entry.perms.length)

// Matrix columns, left → right. `creator` / `moderator` / `admin` are editable;
// `ren` (莲) is the LOCKED safety anchor — always the full catalogue, never
// adjustable, and never PUT.
export interface KunPermRoleColumn {
  role: string
  label: string
  editable: boolean
  locked: boolean
}

export const KUN_PERM_ROLE_COLUMNS: KunPermRoleColumn[] = [
  { role: 'creator', label: '创作者', editable: true, locked: false },
  { role: 'moderator', label: '版主', editable: true, locked: false },
  { role: 'admin', label: '管理员', editable: true, locked: false },
  { role: 'ren', label: '莲', editable: false, locked: true }
]

// The roles a PUT may target. `ren` is intentionally absent.
export const KUN_PERM_EDITABLE_ROLES = [
  'creator',
  'moderator',
  'admin'
] as const

// Delegation rank ladder — MUST stay in lockstep with apps/api/pkg/perm/rank.go
// (roleRank). Order high → low: ren=4, admin=3, moderator=2, creator=1; any
// other/absent role (incl. the implicit `user`) = 0. Used ONLY by the permission
// editors' delegation guard: an operator may edit a subject strictly BELOW their
// own rank. This is a LOCAL ordering for delegation authority, NOT a general role
// tier — capability checks still go through useCan, never a number. The backend
// (pkg/perm) is the real boundary; this mirror is UX only.
export const KUN_ROLE_RANK: Record<string, number> = {
  ren: 4,
  admin: 3,
  moderator: 2,
  creator: 1
}

// The delegation rank of a role SET: the max rank over its claims (0 if empty).
export const kunRoleRank = (roles: string[]): number =>
  roles.reduce((max, role) => Math.max(max, KUN_ROLE_RANK[role] ?? 0), 0)

// The 9 INFRA-PROXY operations, shown read-only. Their truth lives in infra
// (kun-galgame-infra editing engine / kun_trust), NOT in pkg/perm — surfacing
// them here would only invite drift, so admins can see the full picture but
// cannot create an override for them. `taxonomy` is deliberately the stricter
// admin-only threshold per the site owner's standing ruling.
export interface KunProxyPermission {
  key: string
  label: string
  note: string
}

// Keys below mirror the CANONICAL 9-op enumeration in apps/api/pkg/perm's
// package doc (and docs/proj/permissions.md 表一) verbatim — display names must
// not fork from the backend vocabulary.
export const KUN_PROXY_PERMISSIONS: KunProxyPermission[] = [
  {
    key: 'galgame.create',
    label: 'Galgame 直发建条目',
    note: '版主+，绕过提交队列；镜像 NextMoe galgame.create'
  },
  {
    key: 'galgame.review_submission',
    label: 'Galgame 提交审核队列',
    note: '版主+；镜像 NextMoe edit.galgame.game.status'
  },
  {
    key: 'galgame.review',
    label: 'Galgame 提案查看 / 队列',
    note: '版主+ 或条目创建者；镜像 NextMoe galgame.review'
  },
  {
    key: 'galgame.edit_decide',
    label: 'Galgame 提案裁决',
    note: '管理员+ 或条目创建者；镜像 NextMoe edit.galgame.game.review'
  },
  {
    key: 'galgame.edit_revert',
    label: 'Galgame 修订回滚',
    note: '管理员+ 或条目创建者；镜像 NextMoe edit.galgame.game.review / galgame.owner_override'
  },
  {
    key: 'taxonomy.edit',
    label: '资料库条目编辑',
    note: '莲裁定的更严门槛（仅 admin / ren，NextMoe 为版主+）'
  },
  {
    key: 'taxonomy.delete',
    label: '资料库条目删除',
    note: '莲裁定的更严门槛（仅 admin / ren，NextMoe 为版主+）'
  },
  {
    key: 'taxonomy.revert',
    label: '资料库条目回滚',
    note: '莲裁定的更严门槛（仅 admin / ren，NextMoe 为版主+）'
  },
  {
    key: 'trust.queue_access',
    label: 'Trust 举报收件箱',
    note: '版主+；镜像 NextMoe trust.queue_access'
  }
]
