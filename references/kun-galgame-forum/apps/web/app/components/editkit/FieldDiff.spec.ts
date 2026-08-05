// @vitest-environment nuxt
//
// FieldDiff image-hint rendering. The Nuxt environment is needed because the
// component tree reaches for the auto-imported KunChip / KunIcon at render time.
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import FieldDiff from './FieldDiff.vue'

// Mirrors the galgame screenshots/covers config: an item is a row carrying an
// image_hash, and the host resolves it to a URL.
const imageConfig = {
  label: '画廊',
  resolveImage: (v: unknown) =>
    `https://img.test/${(v as { image_hash: string }).image_hash}.webp`,
  formatItem: (v: unknown) => (v as { image_hash: string }).image_hash
}

const shot = (hash: string) => ({ image_hash: hash, sort_order: 0 })

describe('FieldDiff — image hint', () => {
  it('draws only the changed pictures, never the whole gallery', async () => {
    // The reported regression on /galgame/1/history #2 → #3: a revision that
    // ADDS one screenshot. The old two-column view had nothing removed, so the
    // 修改前 column fell back to stringifying the entire old field — every
    // unchanged image rendered as a raw URL.
    const kept = ['a', 'b', 'c', 'd'].map(shot)
    const w = await mountSuspended(FieldDiff, {
      props: {
        label: '画廊',
        diffHint: 'image',
        from: kept,
        to: [...kept, shot('new')],
        config: imageConfig
      }
    })

    const html = w.html()
    // The one added picture is drawn...
    expect(html).toContain('https://img.test/new.webp')
    // ...and none of the four unchanged ones are, in any form.
    for (const hash of ['a', 'b', 'c', 'd']) {
      expect(html).not.toContain(`https://img.test/${hash}.webp`)
    }
    // They are counted instead.
    expect(w.text()).toContain('4 张未改动')
    expect(w.text()).toContain('+1')
  })

  it('marks removed and added pictures in one strip', async () => {
    const w = await mountSuspended(FieldDiff, {
      props: {
        label: '画廊',
        diffHint: 'image',
        from: [shot('old')],
        to: [shot('new')],
        config: imageConfig
      }
    })
    const html = w.html()
    expect(html).toContain('https://img.test/old.webp')
    expect(html).toContain('https://img.test/new.webp')
    // No 修改前 / 修改后 columns — that split is what this replaced.
    expect(w.text()).not.toContain('修改前')
    expect(w.text()).not.toContain('修改后')
  })

  it('a pure reorder draws nothing and says so', async () => {
    const a = shot('a')
    const b = shot('b')
    const w = await mountSuspended(FieldDiff, {
      props: {
        label: '画廊',
        diffHint: 'image',
        from: [a, b],
        to: [b, a],
        config: imageConfig
      }
    })
    expect(w.html()).not.toContain('https://img.test/')
    expect(w.text()).toContain('仅顺序调整')
  })

  it('scalar imagehash renders both sides of the replace', async () => {
    const w = await mountSuspended(FieldDiff, {
      props: {
        label: '横幅图',
        diffHint: 'image',
        from: 'oldhash',
        to: 'newhash',
        config: {
          label: '横幅图',
          resolveImage: (v: unknown) => `https://img.test/${String(v)}.webp`
        }
      }
    })
    const html = w.html()
    expect(html).toContain('https://img.test/oldhash.webp')
    expect(html).toContain('https://img.test/newhash.webp')
  })

  it('falls back to the item identity, not the field, when unresolvable', async () => {
    // No resolveImage in the config: name the changed item. Dumping the whole
    // field as text is what the old fallback did.
    const w = await mountSuspended(FieldDiff, {
      props: {
        label: '画廊',
        diffHint: 'image',
        from: [shot('a'), shot('b')],
        to: [shot('a'), shot('b'), shot('c')],
        config: { label: '画廊', formatItem: imageConfig.formatItem }
      }
    })
    const text = w.text()
    expect(text).toContain('+ c')
    expect(text).not.toContain('a')
    expect(text).not.toContain('b')
  })
})
