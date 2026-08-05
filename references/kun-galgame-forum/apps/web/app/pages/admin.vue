<script setup lang="ts">
import {
  KUN_ADMIN_PAGE_ASIDE_NAV_ITEM,
  type KUN_ADMIN_PAGE_ROUTE_TYPE
} from '~/constants/admin'

useKunDisableSeo('管理系统')

const route = useRoute()
const pageType = computed(() => {
  const routeType = route.fullPath.split('/').pop()
  return routeType as KUN_ADMIN_PAGE_ROUTE_TYPE
})

// Underlined vertical tab rail (same style as the home feed, one size up).
// Selecting a tab navigates to /admin/<router>; the active tab tracks the route.
const adminNavItems = KUN_ADMIN_PAGE_ASIDE_NAV_ITEM.map((item) => ({
  value: item.router!,
  textValue: item.label,
  icon: item.icon
}))
</script>

<template>
  <div class="flex gap-3">
    <div class="hidden w-48 shrink-0 sm:block">
      <KunTab
        :model-value="pageType"
        :items="adminNavItems"
        orientation="vertical"
        variant="underlined"
        color="primary"
        size="lg"
        full-width
        @update:model-value="(value) => navigateTo(`/admin/${value}`)"
      />
    </div>

    <NuxtPage />
  </div>
</template>
