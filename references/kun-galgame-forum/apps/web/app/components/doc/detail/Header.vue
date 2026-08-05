<script setup lang="ts">
// Editing / deleting a doc now lives only in the admin doc manager
// (/admin/doc); the public detail page is read-only.
const props = defineProps<{
  metadata: DocArticleDetail
}>()

const metadata = computed(() => props.metadata)
</script>

<template>
  <KunCard :is-hoverable="false" class-name="border-none">
    <div class="relative mb-6 aspect-video h-full w-full">
      <!-- Declarative lightbox on the cover (real <KunImage>, not
           v-html). wrap=false so the bare <KunImage> stays the layout
           box that fills the aspect-video container. -->
      <KunLightboxGallery>
        <KunLightboxGalleryItem
          :src="metadata.banner_url || '/kungalgame.webp'"
          :alt="metadata.title"
          :wrap="false"
          v-slot="{ open }"
        >
          <KunImage
            :alt="metadata.title"
            class="size-full cursor-zoom-in rounded-lg object-cover"
            :src="metadata.banner_url || '/kungalgame.webp'"
            loading="eager"
            fetchpriority="high"
            width="100%"
            height="100%"
            @click="open"
          />
        </KunLightboxGalleryItem>
      </KunLightboxGallery>
    </div>

    <div class="flex flex-col gap-3">
      <h1 class="text-2xl font-bold tracking-tight sm:text-4xl">
        {{ metadata.title }}
      </h1>

      <div class="flex flex-wrap items-center gap-3 text-sm">
        <KunChip color="secondary">
          {{ metadata.category?.title || `分类 #${metadata.category_id}` }}
        </KunChip>
        <div class="text-default-500 flex items-center gap-1">
          <KunIcon name="lucide:eye" class="h-4 w-4" />
          <span>{{ metadata.view }} 次浏览</span>
        </div>
      </div>

      <div class="flex items-center gap-3">
        <div class="flex flex-col gap-1">
          <div class="text-default-500 flex items-center gap-2">
            <KunIcon name="lucide:calendar-days" />
            <p class="text-small text-inherit">
              <KunTime
                :time="metadata.published_time"
                type="datetime"
                show-year
              />
            </p>
          </div>
        </div>
      </div>

      <div class="bg-primary/10 text-primary-700 rounded-lg p-3 text-sm">
        {{ metadata.description }}
      </div>
    </div>
  </KunCard>
</template>
