<script setup lang="ts">
// TODO_CREATION / UPDATE_LOG_CREATION — the site's 待办 / 更新日志 entries (both
// link to /update). A typed label (icon + name) with a badge — the changelog
// version, or the todo's completion status — over the content.
import { KUN_UPDATE_LOG_STATUS_MAP } from '~/constants/update'

const props = defineProps<{ activity: ActivityItem }>()

const data = computed(() => props.activity.data as NoteActivityData | undefined)
const isTodo = computed(() => props.activity.type === 'TODO_CREATION')

const meta = computed(() =>
  isTodo.value
    ? { icon: 'lucide:list-checks', label: '待办事项', color: 'text-warning-600' }
    : { icon: 'lucide:megaphone', label: '更新日志', color: 'text-primary' }
)

// TODO → its completion status (colored by state); UPDATE_LOG → its version.
const TODO_STATUS_CLASS: Record<number, string> = {
  0: 'bg-default-100 text-default-600',
  1: 'bg-primary/10 text-primary',
  2: 'bg-success/10 text-success',
  3: 'bg-default-100 text-default-500'
}
const badge = computed<{ text: string; class: string } | null>(() => {
  if (isTodo.value) {
    const s = data.value?.status
    if (s === undefined || s === null) return null
    return {
      text: KUN_UPDATE_LOG_STATUS_MAP[s] ?? '',
      class: TODO_STATUS_CLASS[s] ?? 'bg-default-100 text-default-600'
    }
  }
  return data.value?.version
    ? { text: data.value.version, class: 'bg-primary/10 text-primary' }
    : null
})
</script>

<template>
  <ActivityCardShell :actor="activity.actor" :timestamp="activity.timestamp">
    <KunLink
      underline="none"
      color="default"
      :to="activity.link"
      class-name="group block space-y-1.5"
    >
      <span
        :class="cn('flex items-center gap-1.5 text-sm font-medium', meta.color)"
      >
        <KunIcon :name="meta.icon" class="size-4 shrink-0" />
        {{ meta.label }}
        <span
          v-if="badge"
          :class="
            cn('rounded-full px-1.5 py-0.5 text-xs font-medium', badge.class)
          "
        >
          {{ badge.text }}
        </span>
      </span>
      <p
        class="group-hover:text-primary line-clamp-4 text-base break-all transition-colors"
      >
        {{ markdownToText(activity.content) }}
      </p>
    </KunLink>
  </ActivityCardShell>
</template>
