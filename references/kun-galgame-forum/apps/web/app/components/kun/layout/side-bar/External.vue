<script setup lang="ts">
import { kunSubSites } from '~/constants/layout'
</script>

<template>
  <div class="mt-3 flex flex-col justify-center gap-1 p-3">
    <h2 class="text-default-700 px-4 select-none">子网站</h2>

    <KunLink
      color="default"
      underline="none"
      v-for="(site, index) in kunSubSites"
      target="_blank"
      :to="site.link"
      :key="index"
      class-name="hover:bg-primary/20 gap-0 flex flex-nowrap items-center justify-between rounded-lg px-4 py-2 transition-all hover:opacity-80"
    >
      <span class="mr-3 flex items-center justify-center text-xl text-inherit">
        <KunIcon class="mr-3 text-inherit" :name="site.icon" />
        <span class="text-sm">{{ site.short }}</span>
      </span>

      <span class="text-primary text-xs" v-if="site.hint">
        {{ site.hint }}
      </span>
    </KunLink>
  </div>
</template>
