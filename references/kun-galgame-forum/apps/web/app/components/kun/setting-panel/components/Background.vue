<script setup lang="ts">
import { backgroundImages } from './backgroundImage'

const { showKUNGalgameBackground, showKUNGalgameBackgroundOpacity } =
  storeToRefs(usePersistSettingsStore())

const itemsPerPage = 12
const totalPages = Math.ceil(backgroundImages.length / itemsPerPage)
const currentPage = ref(1)

const paginatedImages = computed(() => {
  const start = (currentPage.value - 1) * itemsPerPage
  return backgroundImages.slice(start, start + itemsPerPage)
})

const prevPage = () => {
  if (currentPage.value > 1) currentPage.value--
}
const nextPage = () => {
  if (currentPage.value < totalPages) currentPage.value++
}

const restoreBackground = async () => {
  await usePersistSettingsStore().setSystemBackground(0)
}
const handleChangeImage = async (index: number) => {
  await usePersistSettingsStore().setSystemBackground(index)
}
</script>

<template>
  <div class="space-y-3">
    <div class="space-y-0.5">
      <p class="text-default-700 font-medium">背景图片</p>
      <p class="text-default-500 text-sm">
        为整站挑选一张背景图，或在右下角上传你自己的图片。
        <span v-if="showKUNGalgameBackground === -1" class="text-primary">
          当前：自定义图片。
        </span>
        <span v-else-if="showKUNGalgameBackground === 0" class="text-primary">
          当前：默认背景。
        </span>
      </p>
    </div>

    <div class="grid grid-cols-3 gap-2">
      <KunTooltip
        v-for="image in paginatedImages"
        :key="image.index"
        :text="image.message['zh-cn']"
        position="bottom"
      >
        <button
          type="button"
          class="group relative block aspect-video w-full overflow-hidden rounded-lg ring-2 transition"
          :class="
            showKUNGalgameBackground === image.index
              ? 'ring-primary'
              : 'ring-transparent hover:ring-default-300'
          "
          @click="handleChangeImage(image.index)"
        >
          <KunImage
            class="size-full object-cover transition-transform duration-300 group-hover:scale-110"
            :src="`bg/bg${image.index}-m.webp`"
            loading="lazy"
          />
          <span
            v-if="showKUNGalgameBackground === image.index"
            class="bg-primary text-primary-foreground absolute top-1 right-1 flex size-5 items-center justify-center rounded-full"
          >
            <KunIcon name="lucide:check" class="size-3.5" />
          </span>
        </button>
      </KunTooltip>
    </div>

    <div class="flex items-center justify-between gap-2">
      <div class="text-default-500 flex items-center gap-1.5 text-sm">
        <KunButton
          :is-icon-only="true"
          variant="flat"
          color="default"
          size="sm"
          :disabled="currentPage === 1"
          @click="prevPage"
        >
          <KunIcon name="lucide:chevron-left" />
        </KunButton>
        <span class="tabular-nums">{{ currentPage }} / {{ totalPages }}</span>
        <KunButton
          :is-icon-only="true"
          variant="flat"
          color="default"
          size="sm"
          :disabled="currentPage === totalPages"
          @click="nextPage"
        >
          <KunIcon name="lucide:chevron-right" />
        </KunButton>
      </div>

      <div class="flex items-center gap-2">
        <KunButton variant="flat" color="default" size="sm" @click="restoreBackground">
          恢复默认
        </KunButton>
        <KunSettingPanelComponentsCustomBackground />
      </div>
    </div>

    <!-- 背景图片透明度 -->
    <div class="space-y-2">
      <div class="flex items-center justify-between">
        <div class="text-default-700 flex items-center gap-2 font-medium">
          <KunIcon class="text-primary" name="lucide:image" />
          <span>背景图片透明度</span>
        </div>
        <span class="text-default-500 text-sm tabular-nums">
          {{ showKUNGalgameBackgroundOpacity }}%
        </span>
      </div>
      <KunSlider
        :min="0"
        :max="100"
        :step="1"
        v-model="showKUNGalgameBackgroundOpacity"
      />
    </div>
  </div>
</template>
