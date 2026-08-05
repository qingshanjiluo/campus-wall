<script setup lang="ts">
import { showMoeMessage } from '~/widget/showMoeMessage'

const {
  showKUNGalgamePageTransparency,
  showKUNGalgameBackgroundBlur,
  showKUNGalgameRounded
} = storeToRefs(usePersistSettingsStore())

// The single global login/register modal. Bound to useAuthModal()'s shared
// state so the top-bar 登录 button AND every requireLogin() gate open this one
// instance — mounted at app root so it works on every layout / page.
const { isOpen: isAuthModalOpen } = useAuthModal()

const route = useRoute()
const config = useRuntimeConfig()

useHead({
  htmlAttrs: { lang: 'zh-Hans' },
  meta: [
    {
      name: 'image',
      content: kungal.images[0] ? kungal.images[0].fullUrl : '/kungalgame.webp'
    },
    {
      name: 'yandex-verification',
      content: config.public.KUN_VISUAL_NOVEL_FORUM_YANDEX_VERIFICATION
    }
  ],
  templateParams: {
    schemaOrg: {
      host: kungal.domain.main,
      path: route.path,
      inLanguage: 'zh-Hans'
    }
  },
  link: [
    {
      rel: 'alternative',
      href: `https://www.kungal.com/rss/topic.xml`,
      type: 'application/rss+xml',
      title: () => `${kungal.titleShort}话题订阅`
    },
    {
      rel: 'feed',
      href: `https://www.kungal.com/rss/topic.xml`,
      type: 'application/rss+xml',
      title: `${kungal.titleShort}话题订阅`
    },
    {
      rel: 'alternative',
      href: `https://www.kungal.com/rss/galgame.xml`,
      type: 'application/rss+xml',
      title: () => `${kungal.titleShort} Galgame 订阅`
    },
    {
      rel: 'feed',
      href: `https://www.kungal.com/rss/galgame.xml`,
      type: 'application/rss+xml',
      title: () => `${kungal.titleShort} Galgame 订阅`
    },
    {
      rel: 'me',
      href: `https://mastodon.social/@kungal`,
      type: 'text/html',
      title: 'Mastodon'
    },
    { rel: 'icon', type: 'image/x-icon', href: '/favicon.ico' },
    { rel: 'canonical', href: kungal.domain.main }
  ]
})

useSeoMeta({
  titleTemplate: () => kungal.titleTemplate,
  charset: 'utf-8',
  viewport: 'width=device-width, initial-scale=1',
  formatDetection: 'telephone=no',
  description: kungal.description,
  themeColor: kungal.themeColor,

  ogDescription: kungal.description,
  ogLocale: 'zh_CN',
  ogTitle: kungal.title,
  ogSiteName: kungal.title,
  ogType: 'website',

  // use absolute URLs
  ogImage: kungal.images[0]
    ? kungal.images[0].fullUrl
    : `${kungal.domain.main}/kungalgame.webp`,
  ogImageAlt: kungal.title,

  ogImageWidth: 1920,
  ogImageHeight: 1080,
  ogImageType: 'image/png',
  ogImageUrl: `${kungal.domain.main}/kungalgame.webp`,

  twitterCard: 'summary_large_image',
  twitterImage: `${kungal.domain.main}/kungalgame.webp`
})

useSchemaOrg([
  defineOrganization({
    name: kungal.titleShort,
    url: kungal.domain.main,
    sameAs: [kungal.github]
  }),
  defineWebSite({ name: kungal.titleShort, description: kungal.description }),
  defineWebPage()
])

onMounted(() => {
  usePersistSettingsStore().setKUNGalgameTransparency(
    showKUNGalgamePageTransparency.value
  )

  usePersistSettingsStore().setKUNGalgameBackgroundBlur(
    showKUNGalgameBackgroundBlur.value
  )

  // Restore the persisted global corner radius — the forum-side CSS multiplier
  // (--kun-radius-scale). KunUI's side is handled reactively above.
  usePersistSettingsStore().setKUNGalgameRounded(showKUNGalgameRounded.value)

  if (process.env.NODE_ENV === 'development') {
    // Disable pinia console info for dev
    localStorage.setItem(
      '__VUE_DEVTOOLS_NEXT_PLUGIN_SETTINGS__dev.esm.pinia__',
      '{"logStoreChanges":false}'
    )
    // Disable umami for dev
    localStorage.setItem('umami.disabled', '1')
  } else {
    showMoeMessage()
  }
})
</script>

<template>
  <div class="kun">
    <KunAlertProvider />
    <KunMessageProvider />
    <KunLoliProvider />

    <KunAuthModal v-model="isAuthModalOpen" />

    <KunCapture />

    <!-- 萌萌点明细 + 退出登录 modals. Opened from the avatar menu via temp-store
         flags; mounted here at the non-scoped app.vue root (not the <style
         scoped> avatar bar) so Vue doesn't warn about stamping a scope id onto
         their <KunModal> teleport root. Closed by default; harmless logged-out. -->
    <LazyKunTopBarMoemoepointLog />
    <LazyKunTopBarLogout />
    <LazyKunTopBarCreatorApply />

    <!-- Single global report modal (triggered by every <ReportButton>). At the
         stable root so it animates on close and survives a triggering popover. -->
    <LazyReportModal />

    <!-- Single global 推话题 modal (triggered by every <TopicFooterUpvote>),
         here for the same reason: the mobile ⋯ menu row that opens it unmounts
         with its popover the instant the dialog is clicked. -->
    <LazyTopicUpvoteModal />

    <KunFloatingBar />

    <LazyTopicReplyPanel />

    <LazyKunSettingPanel />

    <NuxtLoadingIndicator color="var(--color-primary)" />

    <NuxtLayout>
      <NuxtPage />
    </NuxtLayout>
  </div>
</template>

<style>
.kun-page-enter-active,
.kun-page-leave-active {
  transition: all 0.2s ease;
  will-change: transform, opacity;
}

.kun-page-enter-from {
  opacity: 0;
  transform: translateY(20px);
}
.kun-page-enter-to {
  opacity: 1;
  transform: translateY(0);
}

.kun-page-leave-from {
  opacity: 1;
  transform: translateY(0);
}
.kun-page-leave-to {
  opacity: 0;
  transform: translateY(20px);
}
</style>
