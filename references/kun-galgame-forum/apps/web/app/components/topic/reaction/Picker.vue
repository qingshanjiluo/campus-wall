<script setup lang="ts">
// The reaction picker popover content: 点赞 / 点踩 pinned on top (with their
// effect annotations), then the emoji grid. Emits the chosen reaction key.
import {
  KUN_REACTION_LIKE,
  KUN_REACTION_DISLIKE,
  KUN_REACTION_LIKE_NOTE,
  KUN_REACTION_DISLIKE_NOTE,
  KUN_REACTION_EMOJIS,
  reactionAsset
} from '~/constants/reaction'

const props = defineProps<{
  mineKeys?: string[]
  // Topic context only: enables the 查看历史 row (replies don't get it).
  topicId?: number
  // Total reactions on the topic, shown as "xxx 人回应了话题".
  total?: number
}>()
const emit = defineEmits<{ select: [key: string]; viewHistory: [] }>()

const isMine = (key: string) => props.mineKeys?.includes(key) ?? false
</script>

<template>
  <div class="w-72 space-y-2 p-2">
    <!-- Effectful reactions -->
    <button
      type="button"
      :class="
        cn(
          'flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors hover:bg-default-100',
          isMine(KUN_REACTION_LIKE) && 'bg-primary/10'
        )
      "
      @click="emit('select', KUN_REACTION_LIKE)"
    >
      <img
        :src="reactionAsset(KUN_REACTION_LIKE)"
        alt="点赞"
        class="size-7 shrink-0 max-w-none"
      />
      <span class="min-w-0">
        <span class="text-default-800 block text-sm font-medium">点赞</span>
        <span class="text-default-500 block text-xs">{{ KUN_REACTION_LIKE_NOTE }}</span>
      </span>
    </button>

    <button
      type="button"
      :class="
        cn(
          'flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors hover:bg-default-100',
          isMine(KUN_REACTION_DISLIKE) && 'bg-primary/10'
        )
      "
      @click="emit('select', KUN_REACTION_DISLIKE)"
    >
      <img
        :src="reactionAsset(KUN_REACTION_DISLIKE)"
        alt="点踩"
        class="size-7 shrink-0 max-w-none"
      />
      <span class="min-w-0">
        <span class="text-default-800 block text-sm font-medium">点踩</span>
        <span class="text-default-500 block text-xs">{{ KUN_REACTION_DISLIKE_NOTE }}</span>
      </span>
    </button>

    <KunDivider />

    <!-- Emoji grid -->
    <div class="grid max-h-60 grid-cols-6 gap-1 overflow-y-auto">
      <button
        v-for="e in KUN_REACTION_EMOJIS"
        :key="e.key"
        type="button"
        :title="e.label"
        :class="
          cn(
            'flex items-center justify-center rounded-md p-1 transition-colors hover:bg-default-100',
            isMine(e.key) && 'bg-primary/10'
          )
        "
        @click="emit('select', e.key)"
      >
        <img
          :src="reactionAsset(e.key)"
          :alt="e.label"
          class="size-7 max-w-none"
          loading="lazy"
        />
      </button>
    </div>

    <!-- 查看历史 — topic reactions only (left: count, right: open the modal). -->
    <template v-if="topicId">
      <KunDivider />
      <div class="flex items-center justify-between px-1 text-sm">
        <span class="text-default-500">{{ total ?? 0 }} 人回应了话题</span>
        <button
          type="button"
          class="text-primary shrink-0 hover:underline"
          @click="emit('viewHistory')"
        >
          查看历史
        </button>
      </div>
    </template>
  </div>
</template>
