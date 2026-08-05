<script setup lang="ts">
// GALGAME_CREATION — "new galgame" feed card (layout per spec):
//   top    — avatar · username · time (shell), then "创建了一个新的Galgame,已经有
//            N 个下载资源"
//   middle — banner (left) + name · 发售于<date> (right)
//   bottom — 点赞 + 收藏 buttons (reused detail components) + 查看详情 link
// 制作会社 + 简介 are intentionally omitted until the wiki brief exposes them.
const props = defineProps<{ activity: ActivityItem }>()

const data = computed(
  () => props.activity.data as GalgameActivityData | undefined
)
const gid = computed(() => data.value?.galgame_id ?? 0)
const detailLink = computed(() =>
  gid.value ? `/galgame/${gid.value}` : props.activity.link
)

// Hydrate the like/favorite buttons' initial state (the cached feed can't carry
// it); fetched once per session, client-side.
const { isLiked, isFavorited, ensureLoaded } = useMyGalgameInteractions()
onMounted(ensureLoaded)
</script>

<template>
  <ActivityCardShell :actor="activity.actor" :timestamp="activity.timestamp">
    <div class="space-y-3">
      <p class="text-default-600 text-sm">
        创建了一个新的 Galgame,已经有 {{ data?.resource_count ?? 0 }} 个下载资源
      </p>

      <ActivityCardGalgameInfo :activity="activity" />

      <div class="flex items-center gap-2">
        <GalgameLike
          :galgame-id="gid"
          :target-user-id="activity.actor?.id ?? 0"
          :like-count="data?.like_count ?? 0"
          :is-liked="isLiked(gid)"
        />
        <GalgameFavorite
          :galgame-id="gid"
          :target-user-id="activity.actor?.id ?? 0"
          :favorite-count="data?.favorite_count ?? 0"
          :is-favorited="isFavorited(gid)"
        />
        <KunLink
          underline="none"
          color="default"
          :to="detailLink"
          class-name="text-default-500 hover:text-primary ml-auto flex items-center gap-0.5 text-sm"
        >
          查看详情
          <KunIcon name="lucide:chevron-right" class="size-4" />
        </KunLink>
      </div>
    </div>
  </ActivityCardShell>
</template>
