<script setup lang="ts">
// Shared chrome for every feed card: avatar (top-left) + a header line of
// [username → /user/:id/info] [muted time], then the card body via the default
// slot. Centralizes the universal rules — clickable username (darker), lighter
// time (default-500), no type chip — so each card only supplies its body.
defineProps<{
  actor?: KunUser
  timestamp: Date | string
}>()
</script>

<template>
  <div class="flex w-full gap-3">
    <KunAvatar v-if="actor" :user="actor" />

    <div class="min-w-0 flex-1 space-y-2">
      <div class="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm">
        <KunLink
          v-if="actor"
          :to="`/user/${actor.id}`"
          underline="none"
          color="default"
          class-name="text-default-800 hover:text-primary font-medium"
        >
          {{ actor.name }}
        </KunLink>
        <span class="text-default-500">
          <KunTime :time="timestamp" />
        </span>
        <!-- Optional extra header meta after the time (e.g. an edit indicator). -->
        <slot name="meta" />
      </div>

      <slot />
    </div>
  </div>
</template>
