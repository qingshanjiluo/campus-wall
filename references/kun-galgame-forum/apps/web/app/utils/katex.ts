import katex from 'katex'

// The API's markdown pipeline (apps/api goldmark + goldmark-mathjax) emits math
// as literal TeX wrapped in a classified span — nothing typesets it, so readers
// would otherwise see the raw `\(x^2\)`:
//   inline   <span class="math inline">\(  …TeX…  \)</span>
//   display  <span class="math display">\[ …TeX…  \]</span>
// We typeset each span with katex.renderToString. That call is pure, so it
// produces identical HTML on the SSR (Node) and client passes → no hydration
// mismatch and no flash of raw `\(…\)`. KaTeX's CSS/fonts are already loaded
// globally (app/styles/editor/katex.css), so the output is styled immediately.
//
// We target the .math spans goldmark already classified instead of scanning the
// whole string for `\( … \)`, which keeps `\(` inside fenced code blocks
// untouched (goldmark never wraps code in a .math span).
const MATH_SPAN = /<span class="math (inline|display)">([\s\S]*?)<\/span>/g

// bluemonday (the server-side sanitizer) HTML-escapes & / < / > in text nodes,
// so a matrix's `a & b` reaches us as `a &amp; b`. Undo that before handing the
// source to KaTeX. (A raw `<`/`>` written directly against a letter — `$a<b$` —
// is already mangled upstream by the sanitizer's tag parser; write `\lt`/`\gt`
// or add spaces. That predates, and is independent of, this typesetting step.)
const ENTITIES: Record<string, string> = {
  '&amp;': '&',
  '&lt;': '<',
  '&gt;': '>',
  '&quot;': '"',
  '&#34;': '"',
  '&#39;': "'"
}
const unescapeHtml = (s: string): string =>
  s.replace(/&(?:amp|lt|gt|quot|#34|#39);/g, (m) => ENTITIES[m] ?? m)

/**
 * Typeset the math that goldmark-mathjax left in server-rendered content HTML.
 * Pass an API `*Html` field straight through; content with no math is returned
 * untouched (cheap substring check) so math-free pages pay almost nothing.
 */
export const renderKatex = (html: string | null | undefined): string => {
  if (!html || !html.includes('class="math')) {
    return html ?? ''
  }
  return html.replace(MATH_SPAN, (original, kind: string, body: string) => {
    const tex = unescapeHtml(
      body
        .trim()
        .replace(/^\\[([]/, '') // strip the leading \( or \[
        .replace(/\\[)\]]$/, '') // strip the trailing \) or \]
        .trim()
    )
    if (!tex) {
      return original
    }
    try {
      return katex.renderToString(tex, {
        displayMode: kind === 'display',
        // Render a parse error inline (red) instead of throwing — a single bad
        // formula must not blank the whole post.
        throwOnError: false
      })
    } catch {
      return original
    }
  })
}
