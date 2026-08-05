import { differenceInSeconds, differenceInHours, format } from 'date-fns'

// isMeaningfulDate rejects the inputs that otherwise render as garbage:
//   - missing / empty / unparseable     → new Date(...) is Invalid → NaN
//   - Go time.Time zero value           → "0001-01-01T00:00:00Z" (year ≤ 1)
// Both show up post-migration (empty resource_update_time from wiki search, a
// zero `created`, etc.) and must not surface as "NaN 年前" / "2025 年前" / "0001-01-01".
const isMeaningfulDate = (d: Date): boolean =>
  !isNaN(d.getTime()) && d.getFullYear() > 1

export const formatTimeDifference = (pastTime: number | Date | string) => {
  const past = new Date(pastTime)
  if (!isMeaningfulDate(past)) {
    return ''
  }
  const now = new Date()
  const diffInSeconds = differenceInSeconds(now, past)

  if (diffInSeconds < 10) {
    return '数秒前'
  }

  if (diffInSeconds < 60) {
    return `${diffInSeconds} 秒前`
  } else if (diffInSeconds < 3600) {
    const minutes = Math.floor(diffInSeconds / 60)
    return `${minutes} 分钟前`
  } else if (diffInSeconds < 86400) {
    const hours = Math.floor(diffInSeconds / 3600)
    return `${hours} 小时前`
  } else if (diffInSeconds < 2592000) {
    const days = Math.floor(diffInSeconds / 86400)
    return `${days} 天前`
  } else if (diffInSeconds < 31536000) {
    const months = Math.floor(diffInSeconds / 2592000)
    return `${months} 个月前`
  } else {
    const years = Math.floor(diffInSeconds / 31536000)
    return `${years} 年前`
  }
}

export const hourDiff = (upvoteTime: number | Date | string, hours: number) => {
  if (upvoteTime === 0 || upvoteTime === undefined) {
    return false
  }

  const currentTime = new Date()
  const time = new Date(upvoteTime)

  return differenceInHours(currentTime, time) <= hours
}

export const formatDate = (
  time: Date | string | number,
  config?: { isShowYear?: boolean; isPrecise?: boolean }
): string => {
  let formatString = 'MM-dd'

  if (config?.isShowYear) {
    formatString = 'yyyy-MM-dd'
  }

  if (config?.isPrecise) {
    formatString = `${formatString} - HH:mm`
  }

  // date-fns `format` THROWS "Invalid time value" on an unparseable date and
  // happily prints "0001-01-01" for the Go zero value — guard both.
  const d = new Date(time)
  if (!isMeaningfulDate(d)) {
    return ''
  }
  return format(d, formatString)
}

// toYMD normalizes any incoming date string to "YYYY-MM-DD" (or "" for
// missing values). Real-world inputs:
//
//   • Wiki API: ISO datetime ("2016-11-25T00:00:00Z") — most rows
//   • Hand-edited wire payloads: bare "YYYY-MM-DD"
//   • Partial dates ("2024-06"): preserved verbatim
//   • Empty / null: returns ""
//
// Why not reuse formatDate above: formatDate goes through `new Date()`,
// which (a) silently expands partials like "2024-06" into June 1 and
// (b) shifts dates by the viewer's timezone when the input is a UTC
// ISO string. Both behaviors are wrong for release-date display.
// Direct string surgery is deterministic and TZ-free.
//
// Without this normalizer the rewrite form seeds itself with the raw
// ISO string and `release_date` validation (which accepts only
// "" | YYYY-MM-DD) bounces the save with "发售日期格式应为 YYYY-MM-DD
// 或留空"; the detail page also renders the raw ISO.
export const toYMD = (raw?: string | null): string => {
  if (!raw) return ''
  const trimmed = String(raw).trim()
  if (!trimmed) return ''

  if (/^\d{4}-\d{2}-\d{2}$/.test(trimmed)) {
    return trimmed
  }

  // ISO datetime ("YYYY-MM-DDTHH:mm:ssZ") — slice the leading date
  // straight off the source string.
  const isoPrefix = trimmed.match(/^(\d{4}-\d{2}-\d{2})/)
  if (isoPrefix) {
    return isoPrefix[1]!
  }

  // Partial / unknown formats: pass through unchanged so legacy
  // "YYYY-MM" entries keep displaying as "YYYY-MM".
  return trimmed
}

// Single source of truth for rendering a galgame's release date.
// Wire contract (U1): { release_date: string | null, release_date_tba: bool }.
//
// - nil/empty + TBA=false → "未公布"
// - nil/empty + TBA=true  → "未定 (TBA)"
// - "YYYY-MM-DD" + TBA=false → "YYYY-MM-DD"
// - "YYYY-MM-DD" + TBA=true  → "预计 YYYY-MM-DD"  ← TBA may coexist
//                                                   with a predicted
//                                                   date (wiki design).
//
// The date is normalized via toYMD first so ISO datetimes from the
// API render cleanly and never leak the time-of-day to the UI.
export const getReleaseDateText = (
  releaseDate?: string | null,
  releaseDateTBA?: boolean
): string => {
  const d = toYMD(releaseDate)
  if (!d) return releaseDateTBA ? '未定 (TBA)' : '未公布'
  return releaseDateTBA ? `预计 ${d}` : d
}
