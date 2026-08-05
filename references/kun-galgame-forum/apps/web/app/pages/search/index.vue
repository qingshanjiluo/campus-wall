<script setup lang="ts">
// noindex on purpose: search-result pages are thin/duplicate content that
// shouldn't be indexed (Google's own guidance), and search now surfaces ALL
// games regardless of NSFW (the content_limit filter was removed) — so an
// indexable /search could leak R18 entries into search engines. R18 keeps out
// of the index at the page layer (here + the conditional noindex on R18 detail
// pages), NOT by filtering search results. moyu's /search already does this.
useKunDisableSeo('搜索 Galgame')

// NOT keepalive on purpose: keepalive + the global `out-in` pageTransition left
// the deactivated /search page in the DOM on navigation — its search box stuck at
// the bottom of the next page. We preserve the search across navigation via the
// temp store (keywords) + the URL (?type=) and re-fetch on return instead, so
// there's no cached page DOM that can leak. (See SearchBox / SearchContainer.)
</script>

<template>
  <SearchContainer />
</template>
