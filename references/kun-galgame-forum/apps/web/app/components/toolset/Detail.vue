<script setup lang="ts">
import {
  KUN_GALGAME_TOOLSET_TYPE_MAP,
  KUN_GALGAME_TOOLSET_LANGUAGE_MAP,
  KUN_GALGAME_TOOLSET_PLATFORM_MAP,
  KUN_GALGAME_TOOLSET_VERSION_MAP
} from '~/constants/toolset'
import { toolsetUpdateForm } from './rewriteStore'

const props = defineProps<{
  id: number
  toolset: ToolsetDetail
}>()

const { id: userId } = usePersistUserStore()
const canEditAnyToolset = useCan('toolset.edit_any')
const canDeleteAnyToolset = useCan('toolset.delete_any')

const data = computed(() => props.toolset)
// The toolset owner manages their own tool; toolset.edit_any / delete_any
// extend that to anyone's.
const canEditToolset = computed(
  () => data.value.user.id === userId || canEditAnyToolset.value
)
const canDeleteToolset = computed(
  () => data.value.user.id === userId || canDeleteAnyToolset.value
)

// Own the resource list locally so add / edit / delete are reactive. The detail
// fetch (useKunFetch) returns a SHALLOW data ref (Nuxt's default deep:false), so
// mutating the nested props.toolset.resource wouldn't re-render the list — a
// plain ref([...]) is deeply reactive and decoupled from the fetch.
const resources = ref<ToolsetResource[]>([...props.toolset.resource])

const isDeleting = ref(false)
const handleDeleteToolset = async () => {
  if (!userId) {
    useAuthModal().open()
    return
  }

  const moemoePointToConsume = 3 + resources.value.length * 3
  const res = await useComponentMessageStore().alert(
    '确定删除该工具？',
    `删除这个工具将会消耗 ${moemoePointToConsume} 萌萌点, 计算公式为 3 + 这个工具下所有的资源数 * 3, 删除是永久性的, 不可撤销`
  )
  if (!res) {
    return
  }

  isDeleting.value = true
  const ok = await kunFetch(`/toolset/${data.value.id}`, {
    method: 'DELETE',
    query: { toolset_id: data.value.id }
  })
  isDeleting.value = false

  if (ok) {
    useMessage('已删除该工具', 'success')
    navigateTo('/toolset')
  }
}

const handleRewriteToolset = () => {
  toolsetUpdateForm.toolset_id = data.value.id
  toolsetUpdateForm.name = data.value.name
  toolsetUpdateForm.description = data.value.content_markdown
  toolsetUpdateForm.language = data.value.language as 'zh-cn'
  toolsetUpdateForm.platform = data.value.platform as 'windows'
  toolsetUpdateForm.type = data.value.type as 'others'
  toolsetUpdateForm.version = data.value.version as 'rc'
  toolsetUpdateForm.homepage = data.value.homepage
  toolsetUpdateForm.aliases = data.value.aliases
  navigateTo('/edit/toolset/rewrite')
}

const showResourceModal = ref(false)
const handlePublishResource = () => {
  showResourceModal.value = true
}

const isSubmittingRate = ref(false)
const practicalityData = ref<ToolsetRating | null>(null)

const loadPracticalityMine = async () => {
  const res = await kunFetch<ToolsetRating>(
    `/toolset/${props.id}/practicality`,
    {
      method: 'GET',
      query: { toolset_id: props.id }
    }
  )
  if (res) {
    practicalityData.value = res
  }
}

onMounted(loadPracticalityMine)

const handleSetStar = async (val: number) => {
  if (!userId) {
    useAuthModal().open()
    return
  }
  isSubmittingRate.value = true
  await kunFetch(`/toolset/${props.id}/practicality`, {
    method: 'PUT',
    body: { rate: val }
  })

  useMessage(`已评分: ${val} 星`, 'success')
  isSubmittingRate.value = false
  await loadPracticalityMine()
}

const handleResourceAdded = (res: ToolsetResource) => {
  resources.value.push(res)
}

const handleResourceDeleted = (toolsetResourceId: number) => {
  resources.value = resources.value.filter((r) => r.id !== toolsetResourceId)
}

const handleResourceUpdated = (res: ToolsetResource) => {
  const idx = resources.value.findIndex((r) => r.id === res.id)
  if (idx !== -1) {
    resources.value[idx] = res
  }
}
</script>

<template>
  <div class="space-y-4">
    <KunCard
      :is-hoverable="false"
      :is-transparent="false"
      content-class="space-y-6"
    >
      <div class="space-y-3">
        <h1 class="text-2xl leading-tight font-bold">{{ data.name }}</h1>
        <div class="mt-2 flex flex-wrap items-center gap-2">
          <KunChip color="secondary" size="sm">
            {{ KUN_GALGAME_TOOLSET_VERSION_MAP[data.version] }}
          </KunChip>
          <KunChip color="success" size="sm">
            {{ KUN_GALGAME_TOOLSET_PLATFORM_MAP[data.platform] }}
          </KunChip>
          <KunChip color="primary" size="sm">
            {{ KUN_GALGAME_TOOLSET_LANGUAGE_MAP[data.language] }}
          </KunChip>
          <KunChip color="danger" size="sm">
            {{ KUN_GALGAME_TOOLSET_TYPE_MAP[data.type] }}
          </KunChip>
        </div>

        <KunDivider class-name="my-6" />

        <KunContent :content="renderKatex(data.content_html)" />
      </div>

      <!-- min-w-0: this is the clearest case of the `1fr` blowout — the left
           track swaps a fixed-height placeholder for an ApexCharts SVG, which
           sizes itself from its content, so without a floor of 0 the columns
           re-split the moment the chart lands. -->
      <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
        <div class="min-w-0 space-y-3 md:col-span-1">
          <template v-if="practicalityData">
            <ToolsetPracticalityChart
              :practicality-data="practicalityData"
              :id="data.id"
            />
          </template>
          <div v-else class="h-[306px] w-full">
            <KunLoading />
          </div>
        </div>

        <div class="min-w-0 space-y-6 md:col-span-1">
          <div class="space-y-2">
            <h3 class="font-semibold">发布者</h3>
            <KunUserChip :user="data.user" />
          </div>

          <div v-if="data.homepage.length" class="space-y-2">
            <h3 class="font-semibold">主页 / 项目</h3>
            <div class="flex flex-col gap-2">
              <KunLink
                v-for="(url, idx) in data.homepage"
                :key="idx"
                :to="url"
                target="_blank"
                underline="hover"
              >
                {{ url }}
              </KunLink>
            </div>
          </div>

          <div v-if="data.aliases.length" class="space-y-2">
            <h3 class="font-semibold">别名</h3>
            <div class="flex flex-wrap items-center gap-2">
              <KunChip
                v-for="(a, i) in data.aliases"
                :key="i"
                color="default"
                size="sm"
              >
                {{ a }}
              </KunChip>
            </div>
          </div>

          <div class="space-y-2">
            <h3 class="font-semibold">实用性评分</h3>
            <p class="text-default-500 text-sm">点击以评分</p>
            <div v-if="practicalityData" class="flex items-center gap-2">
              <KunRating
                :model-value="practicalityData.mine ?? undefined"
                @set="handleSetStar"
              />
              <KunChip size="sm" variant="flat">
                {{ practicalityData.mine }} / 5
              </KunChip>
            </div>
          </div>
        </div>
      </div>

      <div class="flex flex-wrap items-center justify-between gap-2">
        <div class="text-default-500 text-sm">
          {{ `${formatNumber(data.view)} 浏览` }}
          ·
          {{ `${formatNumber(data.download)} 下载` }}
        </div>
        <div class="flex gap-1">
          <KunButton @click="handlePublishResource">上传 / 添加资源</KunButton>
          <KunButton
            v-if="canEditToolset"
            variant="flat"
            @click="handleRewriteToolset"
          >
            修改
          </KunButton>
          <KunButton
            v-if="canDeleteToolset"
            color="danger"
            variant="flat"
            :loading="isDeleting"
            @click="handleDeleteToolset"
          >
            删除
          </KunButton>
        </div>
      </div>
    </KunCard>

    <KunCard
      :is-hoverable="false"
      :is-transparent="false"
      content-class="space-y-3"
    >
      <ToolsetResourceList
        :toolset-id="data.id"
        :resources="resources"
        @deleted="handleResourceDeleted"
        @updated="handleResourceUpdated"
      />
    </KunCard>

    <KunCard
      :is-hoverable="false"
      :is-transparent="false"
      content-class="space-y-3"
    >
      <ToolsetCommentCommunityContainer :toolset-id="data.id" />
    </KunCard>

    <KunModal
      :model-value="showResourceModal"
      @update:model-value="(v) => (showResourceModal = v)"
      inner-class-name="max-w-2xl"
      :is-dismissable="false"
    >
      <ToolsetResourceContainer
        :toolset-id="data.id"
        @on-close="() => (showResourceModal = false)"
        @on-success="handleResourceAdded"
      />
    </KunModal>
  </div>
</template>
