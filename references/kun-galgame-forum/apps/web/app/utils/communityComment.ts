import type { ForumPermission } from '~/composables/useCan'

// Per-area descriptor for a community-primitive comment section.
//
// All four community-backed comment areas (galgame + rating / website / toolset)
// render through ONE presentational family — CommentCommunityRow /
// CommentCommunityComposer — so they cannot drift apart visually again. Everything
// that genuinely differs between them lives here and only here: the endpoints, the
// server-enforced content cap, the moderation permission key, the DOM anchor
// prefix, and the two per-area behavioural quirks (rating is flat; only galgame
// fans out @mentions).
//
// The galgame area is the reference style, so its entries describe the target
// appearance and the other three conform to it.

export type CommunityCommentTarget =
  | { kind: 'galgame'; galgameId: number }
  | { kind: 'rating'; ratingId: number }
  | { kind: 'website'; websiteId: number; domain: string }
  | { kind: 'toolset'; toolsetId: number }
  | { kind: 'resource'; resourceId: number }
  | { kind: 'quiz'; quizId: number }

export interface CommunityCommentSurface {
  key: CommunityCommentTarget['kind']
  // Content cap, mirroring the handler's validate tag (the single enforcement
  // point is server-side; this only pre-empts a doomed request).
  maxLength: number
  // DOM id prefix for a post row — the deep-link / post-publish scroll anchor.
  anchorPrefix: string
  // Moderation delete key. The author may always delete their own post on top of
  // it, and the resource owner is a server-side superset (charter ruling 20) that
  // deliberately gets no UI entry.
  deletePermission: ForumPermission
  composerPlaceholder: string
  // Rating is FLAT: replies never nest, and every post carries an explicit
  // target_user ("A → B") instead of a parent pointer, so a reply composer must
  // pass the recipient through.
  isFlat: boolean
  // Whether a post renders the "→ 对方" chip. FALSE for galgame: its replies DO
  // carry a server-completed target_user, but that area deliberately retired the
  // 「评论给」affordance in favour of @mentions — rendering the chip there would
  // regress the very surface we are conforming to.
  showsReplyTarget: boolean
  // Whether @mention notifications fan out. Only the galgame create path does the
  // mention fan-out, so the other three must not promise it in their placeholder.
  supportsMentions: boolean
  // GET (list) and POST (create) share one URL per area.
  listUrl: string
  // Merged into the list / create request — website addresses by website_id
  // (its :domain path segment is decorative).
  addressQuery: Record<string, number | string>
  // Edit is a SHARED, region-agnostic post-addressed route across all four areas;
  // only galgame appends ?gid, which keeps its local display counter and mention
  // deep-links in sync.
  editUrl: (postId: number) => string
  editQuery: Record<string, number | string>
  // Delete is region-AWARE for the three resource areas (the resource id is pinned
  // by the path so the server can decide owner authority); galgame reuses the
  // shared post-addressed route.
  deleteUrl: (postId: number) => string
  deleteQuery: Record<string, number | string>
  // Create payload. replyToPostId drives the tree areas; targetUserId is the flat
  // rating area's required recipient (charter ruling 19).
  createBody: (
    content: string,
    replyToPostId: number | null,
    targetUserId?: number
  ) => Record<string, unknown>
}

const MENTION_PLACEHOLDER =
  '请温柔的发表你的看法吧～「评论给」已废除，@用户名 即可通知对方'

// communityCommentSurface resolves a target into its descriptor. Pure — safe to
// call in setup or a computed. A mounted section's target never changes kind, so
// callers may resolve it once at setup.
export const communityCommentSurface = (
  target: CommunityCommentTarget
): CommunityCommentSurface => {
  switch (target.kind) {
    case 'galgame': {
      const base = `/galgame/${target.galgameId}/comments`
      return {
        key: 'galgame',
        maxLength: 5000,
        anchorPrefix: 'galgame-comment',
        deletePermission: 'comment.galgame.delete',
        composerPlaceholder: MENTION_PLACEHOLDER,
        isFlat: false,
        showsReplyTarget: false,
        supportsMentions: true,
        listUrl: base,
        addressQuery: {},
        editUrl: (postId) => `/galgame/comments/${postId}`,
        editQuery: { gid: target.galgameId },
        deleteUrl: (postId) => `/galgame/comments/${postId}`,
        deleteQuery: { gid: target.galgameId },
        createBody: (content, replyToPostId) => ({
          content,
          reply_to_post_id: replyToPostId
        })
      }
    }

    case 'rating':
      return {
        key: 'rating',
        maxLength: 1314,
        anchorPrefix: 'rating-comment',
        deletePermission: 'comment.rating.delete',
        composerPlaceholder: '发布对这个评分的观点，请不要锐评',
        isFlat: true,
        showsReplyTarget: true,
        supportsMentions: false,
        listUrl: `/galgame-rating/${target.ratingId}/comments`,
        addressQuery: {},
        editUrl: (postId) => `/galgame/comments/${postId}`,
        editQuery: {},
        deleteUrl: (postId) =>
          `/galgame-rating/${target.ratingId}/comments/${postId}`,
        deleteQuery: {},
        // Flat area: no parent pointer, an explicit recipient instead.
        createBody: (content, _replyToPostId, targetUserId) => ({
          content,
          target_user_id: targetUserId
        })
      }

    case 'website':
      return {
        key: 'website',
        maxLength: 1007,
        anchorPrefix: 'website-comment',
        deletePermission: 'comment.website.delete',
        composerPlaceholder: '说说你对这个网站的看法吧～',
        isFlat: false,
        showsReplyTarget: true,
        supportsMentions: false,
        listUrl: `/website/${target.domain}/comments`,
        addressQuery: { website_id: target.websiteId },
        editUrl: (postId) => `/galgame/comments/${postId}`,
        editQuery: {},
        deleteUrl: (postId) => `/website/${target.domain}/comments/${postId}`,
        deleteQuery: { website_id: target.websiteId },
        createBody: (content, replyToPostId) => ({
          content,
          website_id: target.websiteId,
          reply_to_post_id: replyToPostId
        })
      }

    case 'toolset':
      return {
        key: 'toolset',
        maxLength: 1007,
        anchorPrefix: 'toolset-comment',
        deletePermission: 'comment.toolset.delete',
        composerPlaceholder: '对这个工具有任何使用疑问，都可以在这里提出～',
        isFlat: false,
        showsReplyTarget: true,
        supportsMentions: false,
        listUrl: `/toolset/${target.toolsetId}/comments`,
        addressQuery: {},
        editUrl: (postId) => `/galgame/comments/${postId}`,
        editQuery: {},
        deleteUrl: (postId) =>
          `/toolset/${target.toolsetId}/comments/${postId}`,
        deleteQuery: {},
        createBody: (content, replyToPostId) => ({
          content,
          reply_to_post_id: replyToPostId
        })
      }

    case 'resource':
      return {
        key: 'resource',
        maxLength: 1007,
        anchorPrefix: 'resource-comment',
        deletePermission: 'comment.resource.delete',
        composerPlaceholder: '这个资源能正常使用吗？有问题可以在这里反馈～',
        isFlat: false,
        showsReplyTarget: true,
        supportsMentions: false,
        listUrl: `/galgame-resource/${target.resourceId}/comments`,
        addressQuery: {},
        editUrl: (postId) => `/galgame/comments/${postId}`,
        editQuery: {},
        deleteUrl: (postId) =>
          `/galgame-resource/${target.resourceId}/comments/${postId}`,
        deleteQuery: {},
        createBody: (content, replyToPostId) => ({
          content,
          reply_to_post_id: replyToPostId
        })
      }

    // The quiz area is spoiler-gated SERVER-side: a viewer who has not answered a
    // concealing quiz receives an empty page with locked=true, and a create is
    // 403'd. The client renders only what that ruling says — it never re-derives
    // the rule (see api service/resource_comment_gate.go).
    case 'quiz':
      return {
        key: 'quiz',
        maxLength: 1007,
        anchorPrefix: 'quiz-comment',
        deletePermission: 'comment.quiz.delete',
        composerPlaceholder: '聊聊这道题目吧～请不要直接剧透答案',
        isFlat: false,
        showsReplyTarget: true,
        supportsMentions: false,
        listUrl: `/galgame-quiz/${target.quizId}/comments`,
        addressQuery: {},
        editUrl: (postId) => `/galgame/comments/${postId}`,
        editQuery: {},
        deleteUrl: (postId) =>
          `/galgame-quiz/${target.quizId}/comments/${postId}`,
        deleteQuery: {},
        createBody: (content, replyToPostId) => ({
          content,
          reply_to_post_id: replyToPostId
        })
      }
  }
}
