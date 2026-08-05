import { describe, it, expect } from 'vitest'
import { getEffectiveBanner } from './getEffectiveBanner'

describe('getEffectiveBanner', () => {
  it('prefers effective_banner_url when present', () => {
    expect(
      getEffectiveBanner({
        effective_banner_url: 'https://cdn/eff.webp',
        banner: 'https://legacy/b.webp'
      })
    ).toBe('https://cdn/eff.webp')
  })

  it('falls back to banner when effective_banner_url is empty', () => {
    expect(
      getEffectiveBanner({
        effective_banner_url: '',
        banner: 'https://legacy/b.webp'
      })
    ).toBe('https://legacy/b.webp')
  })

  it('falls back to banner when effective_banner_url is undefined', () => {
    expect(getEffectiveBanner({ banner: 'https://legacy/b.webp' })).toBe(
      'https://legacy/b.webp'
    )
  })

  it("returns '' when neither field is present", () => {
    expect(getEffectiveBanner({})).toBe('')
    expect(getEffectiveBanner(null)).toBe('')
    expect(getEffectiveBanner(undefined)).toBe('')
  })

  it('trims whitespace-only fields (treats as missing)', () => {
    expect(
      getEffectiveBanner({
        effective_banner_url: '   ',
        banner: 'https://legacy/b.webp'
      })
    ).toBe('https://legacy/b.webp')
  })
})
