<script setup lang="ts">
const route = useRoute()

const docSlug = computed(() => (route.params.slug as string) || '')

const { data } = await useKunFetch<DocArticleDetail>(
  `/doc/article/${docSlug.value}`
)

if (data.value) {
  useKunSeoMeta({
    title: data.value.title,
    description: data.value.description,
    ogImage: data.value.banner_url,
    ogType: 'article',
    articleAuthor: [`${kungal.domain.main}/user/${data.value.author_id}`],
    articlePublishedTime: data.value.published_time?.toString(),
    articleModifiedTime: data.value.edited_time?.toString()
  })
} else {
  useKunDisableSeo('未找到该文档')
}
</script>

<template>
  <div v-if="data" class="min-h-[calc(100dvh-6rem)] pb-6">
    <div class="flex">
      <DocDetailCategoryTree />

      <article class="min-w-0 flex-1 space-y-6 pl-0 lg:pr-67 xl:pl-67">
        <DocDetailHeader :metadata="data" />
        <KunContent :content="renderKatex(data.content_html)" />
        <DocDetailFooter />
      </article>

      <div v-if="data.toc?.length" class="hidden lg:block">
        <div class="fixed -translate-x-67">
          <DocDetailTableOfContent :links="data.toc" />
        </div>
      </div>
    </div>
  </div>
</template>
