<script setup lang="ts">
const props = defineProps<{
  user: UserInfo
}>()

useKunDisableSeo(`${props.user.name} 的工具`)

useHead({
  link: [
    {
      rel: 'canonical',
      href: `${kungal.domain.main}/user/${props.user.id}/toolset`
    }
  ]
})
</script>

<template>
  <UserToolset :user-id="user.id" />
</template>
