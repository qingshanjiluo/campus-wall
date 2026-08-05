<script setup lang="ts">
const props = defineProps<{
  message: MessageSystemMessage
}>()

// System broadcasts are admin-authored HTML (admin-only write path), rendered
// directly. No client-side DOM sanitizer here on purpose — the old DOMPurify
// ran jsdom on every SSR render and leaked the web container to OOM. If
// user-influenced text ever reaches a system message, sanitize it server-side
// via the goldmark + bluemonday pipeline (internal/infrastructure/markdown).
const messageHtml = computed(() => props.message.content['zh-cn'] ?? '')
</script>

<template>
  <div
    class="space-y-3 rounded-lg p-2"
    :class="message.is_read ? 'message-read' : ''"
  >
    <div class="flex items-center break-all">
      <div class="ml-2 text-lg">
        <KunIcon
          class="text-secondary"
          v-if="!message.is_read"
          name="lucide:info"
        />
        <KunIcon
          class="text-default"
          v-if="message.is_read"
          name="lucide:check-check"
        />
      </div>
      <KunAvatar :disable-floating="true" :user="message.admin" />
      <span class="text-default-500 text-sm">
        <KunTime :time="message.created" />
      </span>
    </div>

    <div class="leading-8 break-all" v-html="messageHtml" />
  </div>
</template>
