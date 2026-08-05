<script setup lang="ts">
import { watchDebounced } from '@vueuse/core'

// Admin-only (admin ⊂ ren): mirrors the API's RequireAdmin gate on the
// /admin/user/:id/content-stats + purge routes (AdminUserCard). UX guard — the
// real boundary is the API.
definePageMeta({ middleware: 'admin' })

useKunDisableSeo('用户内容管理')

// Account details (ban / delete / profile) live in the OAuth admin UI now.
// This page only manages the CONTENT a user has published on kungal — find
// a user, then purge everything they posted here (for spam / ad accounts).
//
// Account-center web app root + /users (the OAuth admin user-management page:
// ban / unban / 注销 / 角色). Read DIRECTLY from oauthFrontendUrl config — same
// rationale as the Email/Password 改密 jumps; don't derive from oauthServerUrl
// (dev runs FE + API on different ports).
const oauthUsersAdminURL = computed(
  () => `${useRuntimeConfig().public.oauthFrontendUrl}/users`
)

const searchQuery = ref('')
const users = ref<SearchResultUser[]>([])
const isSearching = ref(false)

const handleSearch = async () => {
  const keywords = searchQuery.value.trim()
  if (!keywords) {
    users.value = []
    return
  }
  isSearching.value = true
  const res = await kunFetch<{ items: SearchResultUser[]; total: number }>(
    '/search',
    // limit must stay <= the /search endpoint cap (SearchRequest max=12),
    // else the request 400s. Username search rarely needs more; refine the
    // query for a narrower match.
    { method: 'GET', query: { keywords, type: 'user', page: 1, limit: 12 } }
  )
  isSearching.value = false
  users.value = res?.items ?? []
}

watchDebounced(() => searchQuery.value, handleSearch, {
  debounce: 500,
  maxWait: 1000
})
</script>

<template>
  <div>
    <KunHeader
      name="用户内容管理"
      description="此处用于管理用户在本站发布的内容: 搜索用户后, 可一键清除其在 kungal 的全部内容 (话题 / 回复 / 评论 / 评分 / 资源 / 网站 / 工具及一切互动), 主要用于清理广告与 spam 账号。单条内容的编辑与删除请直接在对应页面操作。"
    >
      <template #endContent>
        <KunInput
          v-model="searchQuery"
          type="text"
          placeholder="输入用户名以搜索用户"
        />
      </template>
    </KunHeader>

    <!-- kungal 是 OAuth 的 RP，本地不存账号，故无法在此封禁 / 删除账号——只能清内容。
         账号本身的操作集中在统一账号后台，给管理员一个显式入口（此前只有代码注释）。 -->
    <KunInfo
      color="info"
      icon="lucide:shield-alert"
      title="封禁 / 注销账号请前往统一账号后台"
      description="账号本身的封禁、解封、注销 (匿名化) 与角色管理由统一身份服务 (OAuth) 集中处理，在那里操作会对所有站点 (kungal / 摸鱼 / 贴纸…) 同时生效；本页仅用于清理用户在本站发布的内容。"
      class-name="mt-6"
    >
      <KunButton
        :href="oauthUsersAdminURL"
        target="_blank"
        size="sm"
        color="info"
        class-name="mt-2"
      >
        <KunIcon name="lucide:external-link" />
        前往统一账号后台管理用户
      </KunButton>
    </KunInfo>

    <div class="mt-6 flex flex-col gap-3">
      <AdminUserCard v-for="user in users" :key="user.id" :user="user" />
    </div>

    <KunLoading v-if="isSearching" />
    <KunNull
      v-if="!isSearching && !users.length && searchQuery.trim()"
      description="未找到匹配的用户"
    />
  </div>
</template>
