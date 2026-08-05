<script setup lang="ts">
import type {
  WithContext,
  SoftwareApplication,
  Person,
  AggregateRating,
  DiscussionForumPosting
} from 'schema-dts'

// Key by path so navigating between two items of this dynamic route remounts
// the page and re-runs setup — the detail fetch uses a static URL + watch:false.
definePageMeta({ key: (route) => route.path })

const route = useRoute()
const id = computed(() => {
  return parseInt((route.params as { id: string }).id)
})

const { data } = await useKunFetch<ToolsetDetail>(
  `/toolset/${id.value}`,
  {
    method: 'GET',
    query: { toolset_id: id.value },
    watch: false
  }
)

const toolset = data.value

if (toolset) {
  const title = `${toolset.name} 资源下载`
  const pageUrl = `${kungal.domain.main}${route.path}`
  const description = markdownToText(toolset.content_markdown).slice(0, 175)

  const osMap: Record<string, string> = {
    windows: 'Windows',
    linux: 'Linux',
    mac: 'macOS',
    macos: 'macOS'
  }
  const operatingSystem =
    osMap[(toolset.platform || '').toLowerCase()] || toolset.platform || 'All'

  const jsonLdApp: WithContext<
    SoftwareApplication & { aggregateRating?: AggregateRating }
  > = {
    '@context': 'https://schema.org',
    '@type': 'SoftwareApplication',
    name: title,
    alternateName: toolset.aliases || [],
    url: pageUrl,
    description,
    applicationCategory: toolset.type,
    operatingSystem,
    softwareVersion: toolset.version,
    inLanguage: toolset.language,
    datePublished: new Date(toolset.created).toISOString(),
    dateModified: new Date(toolset.updated).toISOString(),
    author: {
      '@type': 'Person',
      name: toolset.user.name
    } as Person,
    sameAs: (toolset.homepage || []).slice(0, 5),
    interactionStatistic: [
      {
        '@type': 'InteractionCounter',
        interactionType: { '@type': 'WatchAction' },
        userInteractionCount: toolset.view || 0
      },
      {
        '@type': 'InteractionCounter',
        interactionType: { '@type': 'DownloadAction' },
        userInteractionCount: toolset.download || 0
      }
    ],
    ...(toolset.practicality_avg && toolset.practicality_count
      ? {
          aggregateRating: {
            '@type': 'AggregateRating',
            ratingValue: Number(toolset.practicality_avg.toFixed(2)),
            ratingCount: toolset.practicality_count,
            reviewCount: toolset.comment_count || toolset.practicality_count,
            bestRating: 5,
            worstRating: 1
          } as AggregateRating
        }
      : {})
  }

  useHead({
    script: [
      {
        id: 'schema-org-software-app',
        type: 'application/ld+json',
        innerHTML: jsonLdApp
      }
    ]
  })

  if (toolset.comment_count && toolset.comment_count > 0) {
    const forumJsonLd: WithContext<DiscussionForumPosting> = {
      '@context': 'https://schema.org',
      '@type': 'DiscussionForumPosting',
      url: pageUrl,
      headline: `${title} - Discussion`,
      articleBody: description,
      datePublished: new Date(toolset.created).toISOString(),
      dateModified: new Date(toolset.updated).toISOString(),
      author: { '@type': 'Person', name: toolset.user.name } as Person,
      commentCount: toolset.comment_count,
      comment: (toolset.comment_preview || []).map((c) => ({
        '@type': 'Comment',
        text: (c.content as string)?.slice(0, 280)?.replace(/\\|\n/g, ''),
        datePublished: new Date(c.created).toISOString(),
        author: { '@type': 'Person', name: c.user.name } as Person
      }))
    }

    useHead({
      script: [
        {
          id: 'schema-org-discussion',
          type: 'application/ld+json',
          innerHTML: forumJsonLd
        }
      ]
    })
  }

  useKunSeoMeta({
    title,
    description,
    articleAuthor: [`${kungal.domain.main}/user/${toolset.user?.id}`],
    articlePublishedTime: toolset.created.toString(),
    articleModifiedTime: toolset.updated.toString()
  })
} else {
  useKunDisableSeo('未找到该工具资源')
}
</script>

<template>
  <div>
    <!-- Single real root box, NOT `display: contents` (see user.vue): a box-less
         root trips Nuxt's "does not have a single root node" and skips the
         page-transition enter, so the page teleports in. -->
    <ToolsetDetail v-if="data" :toolset="data" :id="id" />
    <KunNull v-else description="未找到该工具资源" />
  </div>
</template>
