<script setup lang="ts">
const route = useRoute()

const { showKUNGalgameHamburger, messageStatus } = storeToRefs(
  useTempSettingStore()
)
const { id, moemoepoint, isCheckIn, dailyToolsetUploadBytes } = storeToRefs(
  usePersistUserStore()
)

const { isSnowing, toggleSnow, startSnow } = useKunSnowEffect()

const router = useRouter()
const canGoBack = ref(false)

const updateCanGoBack = () => {
  canGoBack.value = window.history.length > 2
}

watch(
  () => route.name,
  () => {
    useTempSettingStore().reset()
  }
)

onMounted(async () => {
  if (id.value) {
    const result = await kunFetch<{
      moemoepoints: number
      is_check_in: boolean
      has_new_message: boolean
      daily_toolset_upload_bytes: number
    }>('/user/status')
    if (result) {
      isCheckIn.value = result.is_check_in
      moemoepoint.value = result.moemoepoints
      messageStatus.value = result.has_new_message ? 'new' : 'online'
      dailyToolsetUploadBytes.value = result.daily_toolset_upload_bytes
    }
  }

  updateCanGoBack()
  startSnow()
  router.afterEach(() => {
    updateCanGoBack()
  })
})
</script>

<template>
  <div class="flex items-center gap-1">
    <KunButton
      :is-icon-only="true"
      color="default"
      size="xl"
      variant="light"
      @click="showKUNGalgameHamburger = true"
      class-name="flex desktop-nav:hidden"
    >
      <KunIcon name="lucide:menu" />
    </KunButton>

    <KunTooltip :text="canGoBack ? '返回上一页' : '返回主页'" position="bottom">
      <KunButton
        :is-icon-only="true"
        color="default"
        size="xl"
        variant="light"
        class-name="hidden sm:block mr-6"
        @click="canGoBack ? router.back() : navigateTo('/')"
      >
        <KunIcon :name="canGoBack ? 'lucide:arrow-left' : 'lucide:home'" />
      </KunButton>
    </KunTooltip>

    <!-- Logged-out GitHub-star promo. Hidden on phones (< md); the ad slot below
         takes this spot on mobile, while desktop keeps showing both. -->
    <KunTooltip
      text="本网站完全开源, 代码完全自主编写, 点击访问 GitHub 仓库为我们点亮 star ⭐"
      position="bottom"
      class-name="hidden md:inline-block"
      v-if="!id"
    >
      <KunButton
        variant="light"
        color="default"
        size="xl"
        target="_blank"
        :href="kungal.github"
        class-name="text-xl"
      >
        <KunIcon name="ant-design:github-filled" />
        <span class="text-sm sm:text-base">GitHub</span>
      </KunButton>
    </KunTooltip>

    <KunAdAIFYIcon />

    <!-- <KunModal
      :model-value="isShowUpdateAvatarModal"
      @update:model-value="(value) => (isShowUpdateAvatarModal = value)"
    >
      <div class="max-w-xl space-y-3">
        <p>之所以有这个丑陋的弹窗, 是因为我想看见大家都用可爱的孩子做头像!</p>
        <p>没有头像黑乎乎的样子太难看了! 这一定是我最初的设计失误!</p>
        <p>
          但是我真的非常希望大家都有一个可爱的头像, 如果是小只可爱软萌 白毛
          女孩子的话就更好了!!!!!!!
        </p>
        <p>
          所以大家快点一下下面的框框更改头像啦,
          如果找不到小只可爱软萌白毛女孩子, 可以前往
          <KunLink target="_blank" :to="kungal.domain.sticker">
            鲲 Galgame 表情包
          </KunLink>
        </p>
        <LazyUserSettingAvatar />
      </div>
    </KunModal> -->

    <LazyKunTopBarHamburger />
  </div>
</template>
