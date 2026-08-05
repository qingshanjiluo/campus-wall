// Centralised claim-state presentation. Every consumer (publish wizard,
// my-submissions list, draft editor, moderation queue, notifications) renders
// an entry's lifecycle the same way, and a new state only needs touching here.
//
// This replaces the wiki status integers, and the vocabulary is NOT a rename of
// them. Two differences are load-bearing:
//
//   - `draft` covers what the wiki called "VNDB 草稿" AND, until the projector
//     separates them, other people's submissions awaiting review. The label
//     therefore says what the state actually means — an entry nobody has
//     published — instead of asserting a provenance it cannot know.
//   - `pending` is a state of its own now, so "审核中" is finally a thing the UI
//     can say truthfully rather than a guess dressed up as a badge.
//
// A row's own claim never reaches `none`: that is the registry's word for an
// unclaimed entry, which no kungal surface can act on.

// Subset of KunUIColor actually used for state badges. Assignable to KunChip's
// `color` prop (which is the full KunUIColor union).
type GalgameClaimStateColor =
  | 'default'
  | 'primary'
  | 'success'
  | 'warning'
  | 'danger'

export interface GalgameClaimStateBadge {
  label: string
  color: GalgameClaimStateColor
}

export const CLAIM_STATE_LIVE = 'live'
export const CLAIM_STATE_DRAFT = 'draft'
export const CLAIM_STATE_PENDING = 'pending'
export const CLAIM_STATE_DECLINED = 'declined'
export const CLAIM_STATE_HIDDEN = 'hidden'
export const CLAIM_STATE_NONE = 'none'

// An unknown / absent state falls through to "未知" rather than guessing
// "已发布", which would mislabel an unpublished entry as a public one.
export const galgameClaimStateBadge = (
  state: string | undefined
): GalgameClaimStateBadge => {
  switch (state) {
    case CLAIM_STATE_LIVE:
      return { label: '已发布', color: 'success' }
    case CLAIM_STATE_DRAFT:
      return { label: '草稿', color: 'primary' }
    case CLAIM_STATE_PENDING:
      return { label: '审核中', color: 'warning' }
    case CLAIM_STATE_DECLINED:
      return { label: '已拒绝', color: 'danger' }
    case CLAIM_STATE_HIDDEN:
      return { label: '已下架', color: 'default' }
    default:
      return { label: '未知', color: 'default' }
  }
}

// A draft is claimable — nobody has published it. A pending one is not: it is
// somebody else's submission, and the registry refuses the transition.
export const isClaimableState = (state: string | undefined): boolean =>
  state === CLAIM_STATE_DRAFT

// Whether an entry is publicly reachable, i.e. whether linking to its page will
// land on something.
export const isPublicState = (state: string | undefined): boolean =>
  state === CLAIM_STATE_LIVE
