<script setup lang="ts">
defineProps<{
  room: ChatMessageAsideItem
}>()
</script>

<template>
  <KunLink
    color="default"
    underline="none"
    class-name="hover:bg-primary/20 flex cursor-pointer flex-nowrap gap-3 rounded-lg p-2 transition-colors hover:opacity-80"
    :to="`/message/user/${room.route}`"
  >
    <KunAvatar
      :user="{
        id: parseInt(room.route),
        name: room.title,
        avatar: room.avatar
      }"
      size="xl"
      :disable-floating="true"
      :is-navigation="false"
    />
    <div class="justify-space flex w-full flex-col">
      <div class="flex items-center justify-between">
        <span class="font-bold">{{ room.title }}</span>
        <span class="text-default-500 text-sm" v-if="room.last_message_time">
          <KunTime :time="room.last_message_time" />
        </span>
      </div>

      <div class="flex items-center justify-between text-sm">
        <slot name="system" />
        <span class="line-clamp-1 break-all">
          {{ markdownToText(room.content) }}
        </span>
        <KunChip
          class-name="whitespace-nowrap"
          color="primary"
          v-if="room.unread_count"
        >
          {{ room.unread_count }}
        </KunChip>
        <KunChip
          class-name="whitespace-nowrap"
          color="default"
          v-if="!room.unread_count"
        >
          {{ room.count }}
        </KunChip>
      </div>
    </div>
  </KunLink>
</template>
