<script setup lang="ts">
// Galgame list-card display preferences, surfaced here in the global settings
// panel as well as in the list page's own 显示设置 popover (card/Nav.vue) — both
// bind the SAME persisted stores, so a change in either place takes effect on
// every Galgame list immediately.
const {
  showPlatform,
  showRating,
  showViewLike,
  showLanguage,
  showNsfwBadge,
  showPublisher,
  showJapaneseName,
  isOpenInNewTab
} = storeToRefs(usePersistGalgameCardStore())

const { showKUNGalgameNoResource } = storeToRefs(usePersistSettingsStore())
</script>

<template>
  <div class="space-y-4">
    <p class="text-default-500 text-sm">
      控制 Galgame 列表卡片的展示内容，更改后即时生效于全站所有 Galgame 列表。
    </p>

    <div class="space-y-2.5">
      <p class="text-default-700 text-sm font-semibold">卡片四角</p>
      <div class="grid grid-cols-1 gap-2.5 sm:grid-cols-2">
        <KunSwitch v-model="showPlatform" label="左上角 · 游戏平台" />
        <KunSwitch v-model="showRating" label="右上角 · 总评分" />
        <KunSwitch v-model="showViewLike" label="左下角 · 浏览 / 点赞" />
        <KunSwitch v-model="showLanguage" label="右下角 · 可用语言" />
      </div>
    </div>

    <div class="space-y-2.5">
      <p class="text-default-700 text-sm font-semibold">其它</p>
      <div class="grid grid-cols-1 gap-2.5">
        <KunSwitch v-model="showNsfwBadge" label="显示 NSFW 角标" />
        <KunSwitch v-model="showPublisher" label="底部显示发布者与时间" />
        <KunSwitch v-model="showJapaneseName" label="中文名下显示日语名" />
        <KunSwitch v-model="isOpenInNewTab" label="在新页面打开卡片" />
        <KunSwitch
          v-model="showKUNGalgameNoResource"
          label="显示没有下载资源的 Galgame"
        />
      </div>
    </div>
  </div>
</template>
