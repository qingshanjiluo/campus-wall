<script setup lang="ts">
import {
  GALGAME_NAV_CONFIG,
  type KUN_USER_PAGE_GALGAME_TYPE
} from '~/constants/user'

const props = defineProps<{
  user: UserInfo
}>()

const route = useRoute()
const galgameType = computed(() => {
  const routeType =
    (route.params as { type: string }).type.replace(/-/g, '_') || 'galgame_like'
  return routeType as (typeof KUN_USER_PAGE_GALGAME_TYPE)[number]
})

useKunDisableSeo(
  `${props.user.name}${GALGAME_NAV_CONFIG[galgameType.value]?.text ?? 'Galgame'}的 Galgame`
)
</script>

<template>
  <UserGalgame :user-id="user.id" :type="galgameType" />
</template>
