<script setup lang="ts">
import {
  KUN_GALGAME_RESOURCE_LANGUAGE_MAP,
  KUN_GALGAME_RESOURCE_PLATFORM_MAP,
  KUN_GALGAME_RESOURCE_TYPE_MAP
} from '~/constants/galgame'

const props = defineProps<{
  galgame: GalgameResourceSummary
}>()

const galgameName = computed(() => getPreferredLanguageText(props.galgame.name))

const typeLabels = computed(() => {
  if (!props.galgame.type.length) return ['暂无数据']
  return props.galgame.type.map(
    (type) => KUN_GALGAME_RESOURCE_TYPE_MAP[type] || type
  )
})

const languageLabels = computed(() => {
  if (!props.galgame.language.length) return ['暂无数据']
  return props.galgame.language.map(
    (lang) => KUN_GALGAME_RESOURCE_LANGUAGE_MAP[lang] || lang
  )
})

const platformLabels = computed(() => {
  if (!props.galgame.platform.length) return ['暂无数据']
  return props.galgame.platform.map(
    (platform) => KUN_GALGAME_RESOURCE_PLATFORM_MAP[platform] || platform
  )
})
</script>

<template>
  <KunCard :is-hoverable="false" :is-transparent="false">
    <div class="grid grid-cols-1 gap-3 md:grid-cols-4 lg:grid-cols-5">
      <div class="relative aspect-video md:col-span-2">
        <KunImage
          class="size-full rounded-lg object-cover"
          :src="getEffectiveBanner(galgame)"
          loading="eager"
          fetchpriority="high"
          :thumbhash="resolveBannerThumbhash(galgame)"
          :alt="getPreferredLanguageText(galgame.name)"
        />

        <KunChip
          :color="galgame.content_limit === 'sfw' ? 'success' : 'danger'"
          class-name="absolute top-2 left-2"
          variant="solid"
        >
          {{ props.galgame.content_limit.toUpperCase() }}
        </KunChip>
      </div>

      <div class="flex w-full flex-col gap-3 md:col-span-2 lg:col-span-3">
        <div>
          <h2 class="text-2xl font-bold">
            <KunLink
              underline="none"
              color="default"
              :to="`/galgame/${props.galgame.id}`"
              class-name="text-2xl hover:text-primary transition-colors"
            >
              {{ galgameName }}
            </KunLink>
            <KunChip
              class-name="ml-2 -translate-y-1"
              :color="galgame.age_limit === 'all' ? 'success' : 'danger'"
            >
              {{ galgame.age_limit === 'all' ? '全年龄' : 'R18' }}
            </KunChip>
          </h2>
          <p class="text-default-500 mt-1 text-sm">
            最近更新 <KunTime :time="galgame.resource_update_time" /> ·
            {{ galgame.view.toLocaleString() }} 次浏览
          </p>
        </div>

        <div class="flex flex-wrap gap-3">
          <div>
            <p class="text-default-500 text-xs tracking-wide uppercase">
              支持下载的类型
            </p>
            <div class="mt-1 flex flex-wrap gap-1">
              <KunChip v-for="type in typeLabels" :key="type" variant="flat">
                {{ type }}
              </KunChip>
            </div>
          </div>

          <div>
            <p class="text-default-500 text-xs tracking-wide uppercase">
              支持下载的语言
            </p>
            <div class="mt-1 flex flex-wrap gap-1">
              <KunChip
                v-for="lang in languageLabels"
                :key="lang"
                variant="flat"
              >
                {{ lang }}
              </KunChip>
            </div>
          </div>

          <div>
            <p class="text-default-500 text-xs tracking-wide uppercase">
              支持下载的平台
            </p>
            <div class="mt-1 flex flex-wrap gap-1">
              <KunChip
                v-for="platform in platformLabels"
                :key="platform"
                variant="flat"
              >
                {{ platform }}
              </KunChip>
            </div>
          </div>
        </div>

        <div class="mt-auto flex flex-wrap items-center justify-end gap-2">
          <KunButton variant="flat" href="/galgame"> 浏览更多资源 </KunButton>
          <KunButton :href="`/galgame/${galgame.id}`">
            查看这个 Galgame 的更多资源
          </KunButton>
        </div>
      </div>
    </div>
  </KunCard>
</template>
