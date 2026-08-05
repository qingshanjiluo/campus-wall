<script setup lang="ts">
const props = defineProps<{
  user: UserInfo
}>()

useKunDisableSeo(`${props.user.name} 的动态`)

useHead({
  link: [
    {
      rel: 'canonical',
      href: `${kungal.domain.main}/user/${props.user.id}/activity`
    }
  ]
})
</script>

<template>
  <UserOverview :user-id="user.id" />
</template>
