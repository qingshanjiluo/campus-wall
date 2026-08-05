// @vitest-environment nuxt
//
// uploadGalgameImage covers the FE→kungal multipart wire for U2
// cover/screenshot uploads. We mock the auto-imported kunFetch via
// @nuxt/test-utils so the test asserts the request shape directly
// without an HTTP round-trip.
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { uploadGalgameImage } from './uploadGalgameImage'

// mockNuxtImport is hoisted by a vite transform to the very top of
// the module, BEFORE any module-scope `const` initializers run. Bare
// `const spy = vi.fn()` then passed into the factory closure would
// throw "Cannot access spy before initialization". `vi.hoisted` runs
// alongside the transform, so the spy IS available when the factory
// fires. Standard Nuxt-vitest pattern.
const { fetchSpy } = vi.hoisted(() => ({ fetchSpy: vi.fn() }))
mockNuxtImport('kunFetch', () => fetchSpy)

beforeEach(() => {
  fetchSpy.mockReset()
})

describe('uploadGalgameImage', () => {
  it('POSTs to /image/galgame with multipart FormData carrying file + preset', async () => {
    fetchSpy.mockResolvedValueOnce({
      hash: 'abcd1234567890ab',
      url: 'https://cdn/ab/cd/abcd1234567890ab.webp',
      width: 1920,
      height: 1080,
      size_bytes: 12345,
      deduplicated: false
    })

    const blob = new Blob(['fake-bytes'], { type: 'image/png' })
    const res = await uploadGalgameImage(blob, 'galgame_banner', 'shot.png')

    expect(res?.hash).toBe('abcd1234567890ab')
    expect(res?.url).toContain('.webp')

    expect(fetchSpy).toHaveBeenCalledTimes(1)
    const [path, opts] = fetchSpy.mock.calls[0]!
    expect(path).toBe('/image/galgame')
    expect(opts.method).toBe('POST')

    // Body shape: FormData with `file` + `preset` entries.
    const body = opts.body as FormData
    expect(body).toBeInstanceOf(FormData)
    expect(body.get('preset')).toBe('galgame_banner')
    const filePart = body.get('file')
    expect(filePart).toBeInstanceOf(Blob)
    // The filename third-arg to form.append() is browser/runtime
    // specific in how it's exposed — happy-dom drops it to 'blob'
    // while real browsers serialize the supplied name. We don't
    // assert the round-trip here (it tests the polyfill more than
    // our code); the wire-correctness side is that `file` is
    // present and its content is the blob we passed.
  })

  it('returns null when kunFetch surfaces a business error (returns null)', async () => {
    // kunFetch's response handler maps wiki business errors to null +
    // a toast; uploadGalgameImage must not throw or return undefined.
    fetchSpy.mockResolvedValueOnce(null)
    const blob = new Blob([''], { type: 'image/png' })
    const res = await uploadGalgameImage(blob, 'galgame_banner')
    expect(res).toBeNull()
  })

  it('hits the same endpoint with default filename omitted', async () => {
    // We can't assert the third-arg filename due to happy-dom (see
    // note above); confirm the call still reaches the server with the
    // file + preset fields populated.
    fetchSpy.mockResolvedValueOnce({
      hash: 'aaaa',
      url: 'x',
      width: 0,
      height: 0,
      size_bytes: 0,
      deduplicated: false
    })
    const blob = new Blob([''], { type: 'image/png' })
    await uploadGalgameImage(blob, 'galgame_banner')
    const body = fetchSpy.mock.calls[0]![1].body as FormData
    expect(body.get('preset')).toBe('galgame_banner')
    expect(body.get('file')).toBeInstanceOf(Blob)
  })
})
