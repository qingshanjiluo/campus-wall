// Smoke test — confirms the vitest harness wires happy-dom + globals.
// Keep this file; it's the first thing to fail loud if a future Nuxt /
// vitest upgrade subtly breaks the test runner.
import { describe, it, expect } from 'vitest'

describe('vitest harness', () => {
  it('runs', () => {
    expect(1 + 1).toBe(2)
  })

  it('has a DOM available (happy-dom)', () => {
    const el = document.createElement('div')
    el.textContent = 'ok'
    expect(el.textContent).toBe('ok')
  })
})
