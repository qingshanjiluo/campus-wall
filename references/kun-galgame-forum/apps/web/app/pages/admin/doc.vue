<script setup lang="ts">
import { useSortable, moveArrayElement } from '@vueuse/integrations/useSortable'
import type { KunUIColor } from '@kungal/ui-core'
import type { DocEditorMode } from '~/components/edit/doc/type'

// DocArticleSummary / DocArticleDetail / DocArticleListResponse are
// auto-imported (shared/types).

// Moderator+ (moderator ⊂ admin ⊂ ren): mirrors the API's RequireModerator gate
// on the /admin/doc/article + /doc/* routes. UX guard — the real boundary is the
// API.
definePageMeta({ middleware: 'moderator' })

useKunDisableSeo('文档管理')

// Status (0/1/2) → chip label + color, mirroring the friend-link chip map.
const DOC_STATUS_CHIP: Record<number, { label: string; color: KunUIColor }> = {
  0: { label: '草稿', color: 'default' },
  1: { label: '已发布', color: 'success' },
  2: { label: '已归档', color: 'warning' }
}

// Moderator-gated endpoint that returns ALL statuses in manual order.
const { data, refresh: refetch } = await useKunFetch<DocArticleListResponse>(
  '/admin/doc/article',
  {
    query: { page: 1, limit: 100, order_by: 'order', sort_order: 'asc' }
  }
)

// Drag mutations need a deeply-reactive local copy — useKunFetch's data ref is
// shallow, so reordering it in place would not re-render the list.
const list = ref<DocArticleSummary[]>([...(data.value?.items ?? [])])

const refresh = async () => {
  await refetch()
  list.value = [...(data.value?.items ?? [])]
}

const persistOrder = async () => {
  await kunFetch('/doc/article/reorder', {
    method: 'PUT',
    body: { ids: list.value.map((a) => a.id) }
  })
}

const listEl = ref<HTMLElement | null>(null)
useSortable(listEl, list, {
  animation: 150,
  handle: '.doc-drag-handle',
  onUpdate: (e: { oldIndex?: number; newIndex?: number }) => {
    if (e.oldIndex == null || e.newIndex == null) return
    moveArrayElement(list, e.oldIndex, e.newIndex)
    // Persist the new top→bottom order once Vue settles the moved array.
    nextTick(() => persistOrder())
  }
})

const isModalOpen = ref(false)
const modalMode = ref<DocEditorMode>('create')
const editingArticle = ref<DocArticleDetail | null>(null)

const openCreate = () => {
  modalMode.value = 'create'
  editingArticle.value = null
  isModalOpen.value = true
}

const openEdit = async (row: DocArticleSummary) => {
  // The list rows are summaries; fetch the full detail to pre-fill the editor.
  const detail = await kunFetch<DocArticleDetail>(`/doc/article/${row.slug}`)
  if (!detail) return
  modalMode.value = 'rewrite'
  editingArticle.value = detail
  isModalOpen.value = true
}

const onSaved = async () => {
  isModalOpen.value = false
  await refresh()
}

const handleDelete = async (row: DocArticleSummary) => {
  const confirmed = await useComponentMessageStore().alert(
    `确认删除文档「${row.title}」吗？`,
    '删除操作不可恢复，请慎重。',
    true
  )
  if (!confirmed) return

  const result = await kunFetch('/doc/article', {
    method: 'DELETE',
    query: { article_id: row.id }
  })
  if (result) {
    useMessage('删除文档成功', 'success')
    await refresh()
  }
}

// Quick first-page pin toggle (optimistic; reverts on failure). Uses the
// lightweight pin endpoint so we don't have to re-send the whole article.
const handleTogglePin = async (row: DocArticleSummary, value: boolean) => {
  const prev = row.is_pin
  row.is_pin = value
  const result = await kunFetch('/doc/article/pin', {
    method: 'PUT',
    body: { article_id: row.id, is_pin: value }
  })
  if (result) {
    useMessage(value ? '已置顶' : '已取消置顶', 'success')
  } else {
    row.is_pin = prev
  }
}
</script>

<template>
  <div class="w-full space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold">文档管理</h1>
        <p class="text-default-500 text-sm">
          新建 / 编辑 / 删除文档, 拖拽左侧手柄可调整文档的展示顺序
        </p>
      </div>
      <KunButton color="primary" @click="openCreate">
        <KunIcon name="lucide:plus" />
        新建文档
      </KunButton>
    </div>

    <div ref="listEl" class="space-y-3">
      <KunCard
        v-for="article in list"
        :key="article.id"
        :is-hoverable="false"
        :is-transparent="false"
        padding="sm"
      >
        <div class="flex items-center gap-3">
          <KunIcon
            name="lucide:grip-vertical"
            class="doc-drag-handle text-default-400 shrink-0 cursor-grab active:cursor-grabbing"
          />
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="truncate font-medium">{{ article.title }}</span>
              <KunChip
                :color="DOC_STATUS_CHIP[article.status]?.color ?? 'default'"
              >
                {{ DOC_STATUS_CHIP[article.status]?.label ?? '未知' }}
              </KunChip>
            </div>
            <span class="text-default-500 block truncate text-xs">
              {{ article.category?.title || `分类 #${article.category_id}` }}
            </span>
          </div>
          <div class="flex shrink-0 items-center gap-1.5">
            <span class="text-default-500 text-sm">置顶</span>
            <KunSwitch
              :model-value="article.is_pin"
              color="primary"
              @update:model-value="(v) => handleTogglePin(article, v)"
            />
          </div>
          <KunButton
            size="sm"
            variant="light"
            color="primary"
            @click="openEdit(article)"
          >
            编辑
          </KunButton>
          <KunButton
            size="sm"
            variant="light"
            color="danger"
            @click="handleDelete(article)"
          >
            删除
          </KunButton>
        </div>
      </KunCard>
    </div>

    <KunNull v-if="!list.length" description="暂无文档, 点击右上角新建" />

    <KunModal
      v-model="isModalOpen"
      :is-dismissable="false"
      inner-class-name="max-w-4xl w-full"
      scroll-behavior="inside"
    >
      <EditDocLayout
        v-if="isModalOpen"
        :mode="modalMode"
        :initial-article="editingArticle"
        :redirect-on-success="false"
        @saved="onSaved"
      />
    </KunModal>
  </div>
</template>
