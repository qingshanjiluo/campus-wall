<script setup lang="ts">
defineProps<{
  engine: GalgameEngineItem
}>()
</script>

<template>
  <KunCard
    :is-transparent="false"
    :is-hoverable="true"
    :href="`/galgame/engine/${engine.id}`"
  >
    <h3 class="text-default-900 font-semibold">
      {{ engine.name }}
      <KunChip size="xs">
        {{ `+ ${engine.galgame_count}` }}
      </KunChip>
    </h3>
    <div
      v-if="engine.alias.length"
      class="text-default-500 flex flex-wrap gap-2"
    >
      <KunChip
        size="xs"
        color="success"
        v-for="(a, index) in engine.alias"
        :key="index"
      >
        {{ a }}
      </KunChip>
    </div>
  </KunCard>
</template>
