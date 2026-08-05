// Strip Markdown syntax down to readable plain text. Two modes:
//   • default — flatten to a SINGLE line (newlines → spaces). Right for SEO
//     meta descriptions and one-line list previews, which is almost every
//     caller.
//   • { preserveNewlines: true } — keep the author's line breaks (only blank-
//     line runs are collapsed). Used by the resource-note card, whose <p>
//     renders with `whitespace-pre-line`; without this its line breaks were
//     flattened away ("备注换行失效").
export const markdownToText = (
  markdown: string,
  opts?: { preserveNewlines?: boolean }
) => {
  if (!markdown) return ''
  const stripped = markdown
    // Fenced code blocks: drop the ``` fence lines (incl the language label).
    .replace(/^[ \t]*```+.*$/gm, '')
    // HTML the editor / paste can leave: <br> → line break, then drop any tag
    // (notes like `1. <br />` otherwise showed a literal "<br />" in the card).
    .replace(/<br\s*\/?>/gi, '\n')
    .replace(/<[^>]+>/g, '')
    // Image BEFORE link (an image is a link with a leading `!`) → DROP it
    // entirely: a one-line preview shouldn't surface the image's alt, which is
    // usually the upload filename (e.g. "rtdyhsg-…jpg"). Must run before the
    // link rule, which would otherwise leave a stray "!".
    .replace(/!\[[^\]]*\]\([^)]*\)/g, '')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    // Inline emphasis → inner text: bold, strikethrough, ||spoiler||, italic.
    .replace(/(\*\*|__)(.*?)\1/g, '$2')
    .replace(/~~(.*?)~~/g, '$1')
    .replace(/\|\|(.*?)\|\|/g, '$1')
    .replace(/(\*|_)(.*?)\1/g, '$2')
    // Headings, inline code, horizontal rules.
    .replace(/^\s*#{1,6}\s+(.*)/gm, '$1')
    .replace(/`/g, '')
    .replace(/^\s*(-{3,}|\*{3,}|_{3,})\s*$/gm, '')
    // List markers, GFM task-list checkboxes, blockquote markers.
    .replace(/^\s*([-*+]|\d+\.)\s+/gm, '')
    .replace(/^\s*\[[ xX]\]\s+/gm, '')
    .replace(/^\s*>+\s?/gm, '')
  return (
    opts?.preserveNewlines
      ? stripped.replace(/[ \t]+$/gm, '').replace(/\n{3,}/g, '\n\n')
      : stripped.replace(/\n+/g, ' ')
  ).trim()
}
