<script setup lang="ts">
import { kungal } from '~/config/kungal'

const props = withDefaults(
  defineProps<{
    className?: string
  }>(),
  {
    className: ''
  }
)

// Hidden for any role beyond `user` (版主/管理员/创作者…); see useAdVisible.
const showAd = useAdVisible()
</script>

<template>
  <KunLink
    :class-name="cn(props.className)"
    target="_blank"
    :to="kungal.ad[0]!.link"
    v-if="showAd"
  >
    <KunImage class-name="w-full rounded-lg" :src="kungal.ad[0]!.banner" />
  </KunLink>
</template>
