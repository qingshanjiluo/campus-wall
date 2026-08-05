<script setup lang="ts">
const handleRecover = async () => {
  const res = await useComponentMessageStore().alert(
    '您确定恢复所有设置吗?',
    '这将清除论坛的一切数据, 包括话题的草稿, Galgame 的草稿等, 清除后需要重新登陆。该操作不可撤销。'
  )
  if (!res) {
    return
  }

  await usePersistSettingsStore().setKUNGalgameSettingsRecover()

  useMessage(10109, 'success')
  await navigateTo('/')
}
</script>

<template>
  <div class="space-y-2">
    <div class="space-y-0.5">
      <p class="text-default-700 font-medium">恢复默认设置</p>
      <p class="text-default-500 text-sm">
        清除本地保存的全部论坛设置与草稿（话题、Galgame 草稿等），并退出登录。此操作不可撤销。
      </p>
    </div>
    <KunButton
      full-width
      color="danger"
      variant="flat"
      class-name="p-2"
      @click="handleRecover"
    >
      恢复所有设置为默认
    </KunButton>
  </div>
</template>
