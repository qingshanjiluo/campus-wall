import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import {
  formatTimeDifference,
  hourDiff,
  formatDate,
  toYMD,
  getReleaseDateText
} from './time'

// formatTimeDifference reads `new Date()` for "now"; fake timers pin
// it to a fixed point so the boundary thresholds (10s / 60s / 60m /
// 24h / 30d / 365d) are deterministic across CI / dev hosts.
const NOW = new Date('2026-06-15T12:00:00Z')

beforeEach(() => {
  vi.useFakeTimers()
  vi.setSystemTime(NOW)
})

afterEach(() => {
  vi.useRealTimers()
})

const secsAgo = (n: number) => new Date(NOW.getTime() - n * 1000)
const hoursAgo = (n: number) => secsAgo(n * 3600)
const daysAgo = (n: number) => secsAgo(n * 86400)

describe('formatTimeDifference', () => {
  it('数秒前 when diff < 10s', () => {
    expect(formatTimeDifference(secsAgo(0))).toBe('数秒前')
    expect(formatTimeDifference(secsAgo(9))).toBe('数秒前')
  })

  it('N 秒前 between 10s and 60s', () => {
    expect(formatTimeDifference(secsAgo(10))).toBe('10 秒前')
    expect(formatTimeDifference(secsAgo(59))).toBe('59 秒前')
  })

  it('N 分钟前 between 60s and 1h', () => {
    expect(formatTimeDifference(secsAgo(60))).toBe('1 分钟前')
    expect(formatTimeDifference(secsAgo(3599))).toBe('59 分钟前')
  })

  it('N 小时前 between 1h and 24h', () => {
    expect(formatTimeDifference(hoursAgo(1))).toBe('1 小时前')
    expect(formatTimeDifference(hoursAgo(23))).toBe('23 小时前')
  })

  it('N 天前 between 1d and 30d', () => {
    expect(formatTimeDifference(daysAgo(1))).toBe('1 天前')
    expect(formatTimeDifference(daysAgo(29))).toBe('29 天前')
  })

  it('N 个月前 between 30d and 365d', () => {
    expect(formatTimeDifference(daysAgo(30))).toBe('1 个月前')
    expect(formatTimeDifference(daysAgo(180))).toBe('6 个月前')
  })

  it('N 年前 when diff >= 365d', () => {
    expect(formatTimeDifference(daysAgo(365))).toBe('1 年前')
    expect(formatTimeDifference(daysAgo(800))).toBe('2 年前')
  })

  it('accepts ISO string + epoch ms as input', () => {
    const iso = secsAgo(120).toISOString()
    expect(formatTimeDifference(iso)).toBe('2 分钟前')
    expect(formatTimeDifference(secsAgo(120).getTime())).toBe('2 分钟前')
  })
})

describe('hourDiff', () => {
  it('returns true when within window', () => {
    expect(hourDiff(hoursAgo(2), 5)).toBe(true)
  })

  it('returns false when outside window', () => {
    expect(hourDiff(hoursAgo(10), 5)).toBe(false)
  })

  it('returns false for sentinel zeros (no upvote yet)', () => {
    expect(hourDiff(0, 24)).toBe(false)
    expect(hourDiff(undefined as unknown as number, 24)).toBe(false)
  })
})

describe('formatDate', () => {
  // Use UTC date that local TZ won't shift across day boundaries —
  // pick a mid-day point so HH stays sensible across normal TZ offsets.
  // Assertions are TZ-independent by checking pattern, not exact HH.
  const d = new Date('2026-06-15T12:00:00Z')

  it('default: MM-dd', () => {
    expect(formatDate(d)).toMatch(/^\d{2}-\d{2}$/)
  })

  it('with year: yyyy-MM-dd', () => {
    expect(formatDate(d, { isShowYear: true })).toMatch(/^\d{4}-\d{2}-\d{2}$/)
  })

  it('precise: appends - HH:mm', () => {
    expect(formatDate(d, { isShowYear: true, isPrecise: true })).toMatch(
      /^\d{4}-\d{2}-\d{2} - \d{2}:\d{2}$/
    )
  })

  it('accepts ISO string + epoch ms', () => {
    expect(formatDate('2026-06-15T12:00:00Z', { isShowYear: true })).toMatch(
      /^\d{4}-\d{2}-\d{2}$/
    )
    expect(formatDate(d.getTime(), { isShowYear: true })).toMatch(
      /^\d{4}-\d{2}-\d{2}$/
    )
  })
})

describe('toYMD', () => {
  it('passes "" / null / undefined through as ""', () => {
    expect(toYMD('')).toBe('')
    expect(toYMD(null)).toBe('')
    expect(toYMD(undefined)).toBe('')
    expect(toYMD('   ')).toBe('')
  })

  it('passes bare YYYY-MM-DD verbatim', () => {
    expect(toYMD('2024-06-15')).toBe('2024-06-15')
  })

  it('slices the date portion off an ISO datetime (no TZ shift)', () => {
    expect(toYMD('2016-11-25T00:00:00Z')).toBe('2016-11-25')
    expect(toYMD('2024-12-31T23:59:59+09:00')).toBe('2024-12-31')
  })

  it('keeps partial dates verbatim instead of inventing a day', () => {
    expect(toYMD('2024-06')).toBe('2024-06')
    expect(toYMD('未公布')).toBe('未公布')
  })
})

describe('getReleaseDateText', () => {
  it('未公布 when no date and not TBA', () => {
    expect(getReleaseDateText(null, false)).toBe('未公布')
    expect(getReleaseDateText('', false)).toBe('未公布')
    expect(getReleaseDateText(undefined, undefined)).toBe('未公布')
  })

  it('未定 (TBA) when no date but TBA flagged', () => {
    expect(getReleaseDateText(null, true)).toBe('未定 (TBA)')
    expect(getReleaseDateText('', true)).toBe('未定 (TBA)')
  })

  it('returns the date when present and not TBA', () => {
    expect(getReleaseDateText('2024-06-15', false)).toBe('2024-06-15')
    expect(getReleaseDateText('2024-06-15')).toBe('2024-06-15')
  })

  it('predicted prefix when date present + TBA flagged', () => {
    expect(getReleaseDateText('2024-06', true)).toBe('预计 2024-06')
    expect(getReleaseDateText('2024-06-15', true)).toBe('预计 2024-06-15')
  })

  it('normalizes ISO datetime to YYYY-MM-DD before rendering', () => {
    expect(getReleaseDateText('2016-11-25T00:00:00Z', false)).toBe('2016-11-25')
    expect(getReleaseDateText('2016-11-25T00:00:00Z', true)).toBe(
      '预计 2016-11-25'
    )
  })

  it('trims whitespace-only date strings', () => {
    expect(getReleaseDateText('   ', false)).toBe('未公布')
    expect(getReleaseDateText('   ', true)).toBe('未定 (TBA)')
  })
})
