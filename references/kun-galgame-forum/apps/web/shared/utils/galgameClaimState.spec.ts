import { describe, expect, it } from 'vitest'
import {
  galgameClaimStateBadge,
  isClaimableState,
  isPublicState
} from './galgameClaimState'

describe('galgameClaimStateBadge', () => {
  it('names every state in the registry vocabulary', () => {
    expect(galgameClaimStateBadge('live')).toEqual({
      label: '已发布',
      color: 'success'
    })
    expect(galgameClaimStateBadge('draft')).toEqual({
      label: '草稿',
      color: 'primary'
    })
    expect(galgameClaimStateBadge('pending')).toEqual({
      label: '审核中',
      color: 'warning'
    })
    expect(galgameClaimStateBadge('declined')).toEqual({
      label: '已拒绝',
      color: 'danger'
    })
    expect(galgameClaimStateBadge('hidden')).toEqual({
      label: '已下架',
      color: 'default'
    })
  })

  // Guessing "已发布" here would present an unpublished entry as a public one.
  it('falls through to 未知 rather than guessing', () => {
    expect(galgameClaimStateBadge(undefined)).toEqual({
      label: '未知',
      color: 'default'
    })
    expect(galgameClaimStateBadge('none')).toEqual({
      label: '未知',
      color: 'default'
    })
    expect(galgameClaimStateBadge('nonsense')).toEqual({
      label: '未知',
      color: 'default'
    })
  })

  // The wizard's whole job: offer 认领 on exactly the rows the registry will
  // accept it for. A pending row is somebody else's submission.
  it('only a draft is claimable', () => {
    expect(isClaimableState('draft')).toBe(true)
    expect(isClaimableState('pending')).toBe(false)
    expect(isClaimableState('live')).toBe(false)
    expect(isClaimableState('declined')).toBe(false)
    expect(isClaimableState(undefined)).toBe(false)
  })

  it('only a live entry has a public page to link to', () => {
    expect(isPublicState('live')).toBe(true)
    expect(isPublicState('draft')).toBe(false)
    expect(isPublicState('hidden')).toBe(false)
  })
})
