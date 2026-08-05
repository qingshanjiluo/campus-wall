<script setup lang="ts">
const props = defineProps<{
  user: UserInfo
}>()

useKunSeoMeta({
  title: `${props.user.name} 的主页`,
  description: props.user.bio
    ? `${props.user.name} 的个人主页 —— ${props.user.bio}`
    : `${props.user.name} 在 ${kungal.titleShort} 的个人主页, 查看 TA 发布的话题、评论、收藏与 Galgame 评分。`,
  ...(props.user.avatar ? { ogImage: props.user.avatar } : {})
})

useHead({
  link: [
    {
      rel: 'canonical',
      href: `${kungal.domain.main}/user/${props.user.id}/info`
    }
  ]
})
</script>

<template>
  <UserInfo :user="user" />
</template>
