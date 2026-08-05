<script setup lang="ts">
// The 推话题 records shown below a topic (between the reaction row + the divider):
// who pushed it, their one-liner, and how long ago. Lazily fetched. Each row:
//   <avatar> <name> 推了这个话题，<blurb>            <相对时间>
import { randomUpvoteDescription } from '~/constants/upvote'

interface UpvoteRecord {
  id: number
  user: KunUser
  description: string
  created: string | Date
}

// Two modes: pass `records` directly (the feed card already has them), or a
// `topicId` to lazily fetch them (the topic-detail page).
const props = defineProps<{
  topicId?: number
  records?: UpvoteRecord[]
}>()

const fetched = ref<UpvoteRecord[]>([])
const records = computed(() => props.records ?? fetched.value)

onMounted(async () => {
  if (props.records || !props.topicId) return
  const data = await kunFetch<UpvoteRecord[]>(`/topic/${props.topicId}/upvotes`)
  if (data) fetched.value = data
})

// User's text, else a stable random default (seeded by the record id).
const blurb = (r: UpvoteRecord) => r.description || randomUpvoteDescription(r.id)
</script>

<template>
  <div
    v-if="records.length"
    class="max-h-56 space-y-2 overflow-y-auto pr-1 text-sm"
  >
    <div
      v-for="r in records"
      :key="r.id"
      class="flex items-center justify-between gap-2"
    >
      <div class="flex min-w-0 items-center gap-1.5">
        <KunAvatar :user="r.user" size="sm" />
        <span class="text-default-700 shrink-0 font-medium">
          {{ r.user.name }}
        </span>
        <span class="text-default-500 truncate">
          推了这个话题，<span class="text-secondary font-bold">{{ blurb(r) }}</span>
        </span>
      </div>
      <span class="text-default-400 shrink-0 whitespace-nowrap">
        {{ formatTimeDifference(r.created) }}
      </span>
    </div>
  </div>
</template>
