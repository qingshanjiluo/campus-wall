<script setup lang="ts">
import { kungal } from '~/config/kungal'

// Hidden for any role beyond `user` (版主/管理员/创作者…); see useAdVisible.
const showAd = useAdVisible()
</script>

<template>
  <KunTooltip
    class-name="flex"
    text="听说现在的 AI 比人还要 H ?"
    position="bottom"
    v-if="showAd"
  >
    <KunLink target="_blank" :to="kungal.ad[0]!.link">
      <!-- `block` (was `hidden md:block`) so the ad shows on mobile too, where it
           replaces the GitHub promo in the top bar. -->
      <KunImage class-name="block h-11" :src="kungal.ad[0]!.icon" />
    </KunLink>
  </KunTooltip>
</template>
