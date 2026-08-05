<script setup lang="ts">
const props = defineProps<{
  user: UserInfo
}>()

useKunDisableSeo(`${props.user.name} 的题库`)

useHead({
  link: [
    {
      rel: 'canonical',
      href: `${kungal.domain.main}/user/${props.user.id}/quiz`
    }
  ]
})
</script>

<template>
  <UserQuiz :user-id="user.id" />
</template>
