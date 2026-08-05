<script setup lang="ts">
import type { UpdateWebsiteCategoryPayload } from '~/components/website/modal/types'

// Key by path so navigating between two items of this dynamic route remounts
// the page and re-runs setup — the detail fetch uses a static URL + watch:false.
definePageMeta({ key: (route) => route.path })

const route = useRoute()
const categoryName = computed(() => {
  return (route.params as { name: string }).name
})

const showCategoryModal = ref(false)
const editingCategory = ref<UpdateWebsiteCategoryPayload>(
  {} as UpdateWebsiteCategoryPayload
)

const { data } = await useKunFetch<WebsiteCategoryDetail>(
  `/website-category/${categoryName.value}`,
  {
    watch: false,
    query: { name: categoryName.value }
  }
)

const openEditCategoryModal = () => {
  if (!data.value) {
    return
  }
  editingCategory.value = {
    name: data.value.name,
    label: data.value.label,
    category_id: data.value.id,
    description: data.value.description
  } satisfies UpdateWebsiteCategoryPayload
  showCategoryModal.value = true
}

const handleUpdateCategory = async (data: UpdateWebsiteCategoryPayload) => {
  const result = await kunFetch(`/website-category`, {
    method: 'PUT',
    body: data
  })

  if (result) {
    useMessage('重新编辑成功', 'success')
  }
}

if (data.value) {
  useKunSeoMeta({
    title: data.value.label,
    description: data.value.description,
    articlePublishedTime: data.value.created.toString(),
    articleModifiedTime: data.value.updated.toString()
  })
} else {
  useKunDisableSeo('未找到该网站分类')
}
</script>

<template>
  <div v-if="data" class="space-y-6">
    <KunHeader :name="data.label" :description="data.description">
      <template #endContent>
        <div class="space-y-3">
          <div class="flex items-center space-x-3">
            <KunChip color="primary">
              {{ `本资料库拥有 ${data.website_count} 个 ${data.label}` }}
            </KunChip>
            <KunChip>
              更新于 <KunTime :time="data.updated" type="date" show-year />
            </KunChip>
          </div>

          <div class="flex justify-end">
            <KunButton @click="openEditCategoryModal">编辑分类</KunButton>
          </div>
        </div>
      </template>
    </KunHeader>

    <WebsiteModalCategory
      v-model="showCategoryModal"
      :initial-data="editingCategory"
      @submit="handleUpdateCategory"
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

    <KunNull v-else :description="`${data.label} 分类下暂无网站`" />
  </div>
</template>
