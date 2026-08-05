<script setup lang="ts">
import { friendArray } from '~/config/friend'

const description = `${kungal.titleShort} 的友情链接, 收录 ${friendArray
  .flatMap((group) => group.value.map((friend) => friend.name))
  .join('、')} 等 Galgame 相关的优质网站与社区。`

useKunSeoMeta({
  title: '友情链接网站',
  description,
  articleAuthor: friendArray.flatMap((group) =>
    group.value.map((friend) => friend.link)
  )
})
</script>

<template>
  <FriendLinksContainer />
</template>
