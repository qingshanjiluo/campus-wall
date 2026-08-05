<script setup lang="ts">
// TOOLSET_RESOURCE_CREATION — a user published a resource under a toolset. The
// resource note/content is in content; data.parent_name is the owning toolset.
const props = defineProps<{ activity: ActivityItem }>()

const data = computed(
  () => props.activity.data as EntityRefActivityData | undefined
)
</script>

<template>
  <ActivityCardShell :actor="activity.actor" :timestamp="activity.timestamp">
    <KunLink
      underline="none"
      color="default"
      :to="activity.link"
      class-name="group block space-y-1.5"
    >
      <span class="text-default-500 flex items-center gap-1.5 text-sm">
        <KunIcon
          name="lucide:package-plus"
          class="text-secondary size-4 shrink-0"
        />
        在工具《{{ data?.parent_name }}》发布了资源
      </span>
      <p class="group-hover:text-primary line-clamp-3 text-base break-all transition-colors">
        {{ markdownToText(activity.content) }}
      </p>
    </KunLink>
  </ActivityCardShell>
</template>
