<script setup lang="ts">
const { showKUNGalgameContentLimit } = storeToRefs(usePersistSettingsStore())

const option = ref(showKUNGalgameContentLimit.value === 'nsfw')

watch(
  () => option.value,
  () => {
    const optionString = option.value ? 'nsfw' : 'sfw'
    showKUNGalgameContentLimit.value = optionString
    location.reload()
  }
)
</script>

<template>
  <div class="flex items-start justify-between gap-4">
    <div class="space-y-0.5">
      <p class="text-default-700 font-medium">启用网站 NSFW 模式</p>
      <p class="text-default-500 text-sm">
        显示 R18 等有内容限制的游戏与图片。开启或关闭后会自动刷新页面以重新加载内容。
      </p>
    </div>
    <KunSwitch v-model="option" class="shrink-0" />
  </div>
</template>
