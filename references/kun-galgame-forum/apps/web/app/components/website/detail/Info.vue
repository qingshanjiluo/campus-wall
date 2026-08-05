<script setup lang="ts">
import {
  KUN_WEBSITE_LANGUAGE_MAP,
  KUN_WEBSITE_ACG_LIMIT_MAP
} from '~/constants/galgameWebsite'

defineProps<{
  data: WebsiteDetail
}>()

// Tag the outbound domain jumps with utm_source=<current domain>.
const utmLink = useUtmLink()
</script>

<template>
  <KunCard
    :is-transparent="false"
    :is-hoverable="false"
    class-name="p-6"
  >
    <h3 class="text-default-900 mb-4 text-lg font-semibold">网站信息</h3>
    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <span class="text-default-500 text-sm">分类</span>
        <KunLink
          :to="`/website-category/${data.category.name}`"
          underline="none"
        >
          <KunChip class-name="cursor-pointer" color="primary">
            {{ data.category.label }}
          </KunChip>
        </KunLink>
      </div>

      <div class="flex items-center justify-between">
        <span class="text-default-500 text-sm">语言</span>
        <KunChip color="secondary">
          {{ KUN_WEBSITE_LANGUAGE_MAP[data.language] }}
        </KunChip>
      </div>

      <div class="flex items-center justify-between">
        <span class="text-default-500 text-sm">年龄限制</span>
        <KunChip
          :variant="data.age_limit === 'all' ? 'flat' : 'solid'"
          :color="data.age_limit === 'all' ? 'success' : 'danger'"
        >
          {{ KUN_WEBSITE_ACG_LIMIT_MAP[data.age_limit] }}
        </KunChip>
      </div>

      <div>
        <span class="text-default-500 text-sm">域名列表</span>
        <div
          v-for="(dom, index) in data.domain"
          :key="index"
          class="mt-1 space-x-1"
        >
          <KunLink :to="utmLink(dom)" class-name="font-mono">
            {{ dom }}
          </KunLink>
          <KunButton
            :is-icon-only="true"
            variant="light"
            @click="useKunCopy(dom)"
          >
            <KunIcon name="lucide:copy" />
          </KunButton>
        </div>
      </div>
    </div>
  </KunCard>
</template>
