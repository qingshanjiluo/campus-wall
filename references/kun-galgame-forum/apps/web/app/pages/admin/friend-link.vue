<script setup lang="ts">
import { FRIEND_LINK_CATEGORIES } from '~/constants/friendLink'

// Moderator+ (moderator ⊂ admin ⊂ ren): mirrors the API's RequireModerator gate
// on the /admin/friend-link routes. UX guard — the real boundary is the API.
definePageMeta({ middleware: 'moderator' })

useKunDisableSeo('友链管理')

// Same public endpoint the display page uses; refetched after every mutation.
const { data, refresh } = await useKunFetch<GroupedFriendLinks>('/friend-link')

const isModalOpen = ref(false)
const editing = ref<FriendLink | null>(null)
const modalCategory = ref<FriendLinkCategory>('galgame')

const openAdd = (category: FriendLinkCategory) => {
  editing.value = null
  modalCategory.value = category
  isModalOpen.value = true
}

const openEdit = (link: FriendLink) => {
  editing.value = link
  isModalOpen.value = true
}

const handleSubmit = async (form: FriendLinkInput) => {
  const result = await kunFetch('/admin/friend-link', {
    method: form.id ? 'PUT' : 'POST',
    body: form
  })
  if (result) {
    useMessage(form.id ? '友链已更新' : '友链已添加', 'success')
    await refresh()
  }
}

const handleRemove = async (link: FriendLink) => {
  const confirmed = await useComponentMessageStore().alert(
    `确定删除友链「${link.name}」吗？`
  )
  if (!confirmed) return

  const result = await kunFetch('/admin/friend-link', {
    method: 'DELETE',
    query: { id: link.id }
  })
  if (result) {
    useMessage('友链已删除', 'success')
    await refresh()
  }
}

const handleReorder = async (category: FriendLinkCategory, ids: number[]) => {
  await kunFetch('/admin/friend-link/reorder', {
    method: 'PUT',
    body: { category, ids }
  })
}
</script>

<template>
  <div class="w-full space-y-6">
    <KunHeader
      name="友链管理"
      description="增删改友链, 拖拽左侧手柄可调整每个分类内的展示顺序"
    />

    <AdminFriendLinkSection
      v-for="category in FRIEND_LINK_CATEGORIES"
      :key="category.key"
      :category="category.key"
      :label="category.label"
      :links="data?.[category.key] ?? []"
      @add="openAdd"
      @edit="openEdit"
      @remove="handleRemove"
      @reorder="handleReorder"
    />

    <AdminFriendLinkModal
      v-model="isModalOpen"
      :initial-data="editing"
      :default-category="modalCategory"
      @submit="handleSubmit"
    />
  </div>
</template>
