<script setup lang="ts">
import type {
  CreateWebsiteTagPayload,
  UpdateWebsiteTagPayload
} from '~/components/website/modal/types'

// Key by path so navigating between two items of this dynamic route remounts
// the page and re-runs setup — the detail fetch uses a static URL + watch:false.
definePageMeta({ key: (route) => route.path })

const route = useRoute()
const tagName = computed(() => {
  return (route.params as { name: string }).name
})

const showTagModal = ref(false)
const editingTag = ref<UpdateWebsiteTagPayload>({} as UpdateWebsiteTagPayload)

const { data } = await useKunFetch<WebsiteTagDetail>(
  `/website-tag/${tagName.value}`,
  {
    watch: false,
    query: { name: tagName.value }
  }
)

const openEditTagModal = () => {
  if (!data.value) {
    return
  }
  editingTag.value = {
    name: data.value.name,
    label: data.value.label,
    level: data.value.level,
    tag_id: data.value.id,
    description: data.value.description
  } satisfies UpdateWebsiteTagPayload
  showTagModal.value = true
}

const handleTagSubmit = async (
  data: CreateWebsiteTagPayload | UpdateWebsiteTagPayload
) => {
  if ('tag_id' in data) {
    const result = await kunFetch(`/website-tag`, {
      method: 'PUT',
      body: data
    })

    if (result) {
      useMessage('重新编辑成功', 'success')
    }
  }
}

if (data.value) {
  useKunSeoMeta({
    title: `${data.value.label}的 Galgame 网站`,
    description: data.value.description,
    articlePublishedTime: data.value.created.toString(),
    articleModifiedTime: data.value.updated.toString()
  })
} else {
  useKunDisableSeo('未找到该网站标签')
}
</script>

<template>
  <div v-if="data" class="space-y-6">
    <KunHeader
      :name="`${data.label}的 Galgame 网站`"
      :description="data.description"
    >
      <template #endContent>
        <div class="space-y-3">
          <div class="flex items-center space-x-3">
            <KunChip color="primary">标签价值 {{ data.level }}</KunChip>

            <KunChip>
              更新于 <KunTime :time="data.updated" type="date" show-year />
            </KunChip>
          </div>

          <div class="flex justify-end">
            <KunButton @click="openEditTagModal">编辑标签</KunButton>
          </div>
        </div>
      </template>
    </KunHeader>

    <WebsiteModalTag
      v-model="showTagModal"
      :initial-data="editingTag"
      @submit="handleTagSubmit"
    />

    <div v-if="data.websites.length">
      <div class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
        <WebsiteCard
          v-for="website in data.websites"
          :key="website.id"
          :website="website"
        />
      </div>
    </div>

    <KunNull v-else :description="`${data.label} 标签下暂无网站`" />
  </div>
</template>
