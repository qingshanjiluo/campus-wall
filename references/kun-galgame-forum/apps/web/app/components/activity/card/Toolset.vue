<script setup lang="ts">
// TOOLSET_CREATION — a user created a Galgame toolset. The toolset name is in
// content; the link goes to /toolset/<id>.
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
        <KunIcon name="lucide:wrench" class="text-secondary size-4 shrink-0" />
        创建了工具
      </span>
      <p class="group-hover:text-primary text-lg font-medium break-all transition-colors">
        {{ activity.content }}
      </p>
    </KunLink>
  </ActivityCardShell>
</template>
