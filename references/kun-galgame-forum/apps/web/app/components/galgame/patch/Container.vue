<script setup lang="ts">
import {
  SUPPORTED_TYPE_MAP,
  SUPPORTED_PLATFORM_MAP,
  SUPPORTED_LANGUAGE_MAP
} from './constant'
import type { KunPatchResourceResponse, HikariResponse } from './types'

const props = defineProps<{
  vndbId: string
}>()

const emit = defineEmits<{
  'has-resource': [boolean]
  // Surface the (slow, external moyu.moe) fetch state so the tab panel can dim
  // while it loads.
  'update:loading': [boolean]
}>()

const resources = ref<KunPatchResourceResponse[]>([])
const isLoading = ref(false)
watchEffect(() => emit('update:loading', isLoading.value))

const expandedNotes = ref<Set<number>>(new Set())
const isNoteExpanded = (id: number) => expandedNotes.value.has(id)
const toggleNote = (id: number) => {
  if (expandedNotes.value.has(id)) {
    expandedNotes.value.delete(id)
  } else {
    expandedNotes.value.add(id)
  }
}
const isNoteLong = (note: string) => note.length > 80 || note.includes('\n')

const STORAGE_MAP: Record<string, string> = {
  s3: 'S3 对象存储',
  user: '网盘下载'
}

const fetchKunPatchResource = async (vndbId: string) => {
  isLoading.value = true
  try {
    const data = await fetch(`https://www.moyu.moe/api/hikari?vndb_id=${vndbId}`)
    const res = (await data.json()) as HikariResponse
    if (res.success) {
      resources.value = res.data ? res.data.resource : []
    }
  } catch {
    // moyu.moe is a different origin. If it returns no CORS header the browser
    // blocks the request and `fetch` rejects — and because this runs in
    // onMounted, an unhandled rejection bubbles up and trips the page-level
    // error state ("请求 Galgame 错误"). The patch list is supplementary, so
    // swallow the failure and render the galgame page without it.
  } finally {
    isLoading.value = false
  }
}

onMounted(async () => {
  await fetchKunPatchResource(props.vndbId)
  emit('has-resource', resources.value.length > 0)
})
</script>

<template>
  <div v-if="resources.length" class="flex flex-col gap-3">
    <KunHeader name="补丁资源下载" scale="h2">
      <template #endContent>
        <p class="text-default-500 text-sm">
          下面是从
          <KunLink size="sm" href="https://www.moyu.moe/">
            鲲 Galgame 补丁
          </KunLink>
          获取到的 Galgame 补丁资源, 请杂鱼点击前往本站的补丁网站下载
        </p>
      </template>
    </KunHeader>

    <div class="space-y-2" v-for="resource in resources" :key="resource.id">
      <p v-if="resource.name" class="font-medium break-words">
        {{ resource.name }}
      </p>
      <div class="flex flex-wrap items-center justify-between">
        <div class="flex flex-wrap items-center gap-1 rounded-lg">
          <KunChip
            v-for="(t, index) in resource.type"
            :key="index"
            color="primary"
          >
            {{ SUPPORTED_TYPE_MAP[t] }}
          </KunChip>
          <KunChip color="warning">
            <KunIcon name="lucide:database" />
            {{ resource.size }}
          </KunChip>
          <KunChip
            v-for="(p, index) in resource.platform"
            :key="index"
            color="success"
          >
            {{ SUPPORTED_PLATFORM_MAP[p] }}
          </KunChip>
          <KunChip
            v-for="(l, index) in resource.language"
            :key="index"
            color="secondary"
          >
            {{ SUPPORTED_LANGUAGE_MAP[l] }}
          </KunChip>
          <KunChip v-if="resource.model_name" color="danger">
            <KunIcon name="lucide:bot" />
            {{ resource.model_name }}
          </KunChip>
          <KunChip color="default">
            <KunIcon name="lucide:hard-drive" />
            {{ STORAGE_MAP[resource.storage] ?? resource.storage }}
          </KunChip>
        </div>

        <div class="ml-auto flex items-center gap-1">
          <KunTooltip text="下载数">
            <KunButton
              variant="light"
              color="default"
              size="sm"
              class-name="gap-1"
            >
              <KunIcon name="lucide:download" />
              <span>{{ resource.download }}</span>
            </KunButton>
          </KunTooltip>
        </div>
      </div>

      <KunInfo
        v-if="resource.note"
        color="info"
        variant="flat"
        title="发布者备注 — 请先阅读"
      >
        <div class="space-y-1.5">
          <p
            class="text-sm break-words whitespace-pre-wrap"
            :class="{ 'line-clamp-3': !isNoteExpanded(resource.id) }"
          >
            {{ resource.note }}
          </p>
          <button
            v-if="isNoteLong(resource.note)"
            type="button"
            class="text-default-500 hover:text-primary flex items-center gap-1 px-1 text-xs transition-colors"
            @click="toggleNote(resource.id)"
          >
            <KunIcon
              :name="
                isNoteExpanded(resource.id)
                  ? 'lucide:chevron-up'
                  : 'lucide:chevron-down'
              "
            />
            {{ isNoteExpanded(resource.id) ? '收起' : '展开全部' }}
          </button>
        </div>
      </KunInfo>

      <div class="flex justify-between">
        <div class="flex gap-2">
          <a
            :href="`https://www.moyu.moe/user/${resource.user.id}/resource`"
            target="_blank"
            rel="noopener noreferrer"
            :class="
              cn(
                'flex size-8 shrink-0 cursor-pointer justify-center',
                'hover:ring-primary-500 rounded-full transition duration-150 ease-in-out hover:ring-2'
              )
            "
          >
            <KunImage
              :class="cn('inline-block rounded-full', 'size-8')"
              :src="resource.user.avatar"
              :alt="resource.user.name"
            />
          </a>

          <div class="flex flex-col">
            <span class="text-xs">{{ resource.user.name }}</span>
            <span class="text-default-500 text-xs">
              资源更新于 <KunTime :time="resource.update_time" type="datetime" show-year />
            </span>
          </div>
        </div>

        <KunButton
          size="sm"
          variant="flat"
          target="_blank"
          :href="`https://www.moyu.moe/resource/${resource.id}`"
          :icon="true"
        >
          前往下载页面
          <template #icon>
            <KunIcon name="lucide:external-link" />
          </template>
        </KunButton>
      </div>

      <KunDivider margin="0 0 17px 0" />
    </div>
  </div>
</template>
