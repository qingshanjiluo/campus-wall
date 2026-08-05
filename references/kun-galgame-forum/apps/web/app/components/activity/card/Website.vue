<script setup lang="ts">
// GALGAME_WEBSITE_CREATION — a user added a Galgame website. The website name is
// in content; the link goes to /website/<url>.
defineProps<{ activity: ActivityItem }>()
</script>

<template>
  <ActivityCardShell :actor="activity.actor" :timestamp="activity.timestamp">
    <KunLink
      underline="none"
      color="default"
      :to="activity.link"
      class-name="group block space-y-1"
    >
      <span class="text-default-500 flex items-center gap-1.5 text-sm">
        <KunIcon name="lucide:globe" class="text-secondary size-4 shrink-0" />
        创建了网站
      </span>
      <p class="group-hover:text-primary text-lg font-medium break-all transition-colors">
        {{ activity.content }}
      </p>
    </KunLink>
  </ActivityCardShell>
</template>
