<script setup lang="ts">
const galgame = inject<GalgameDetail>('galgame')
</script>

<template>
  <div v-if="galgame" class="flex items-center justify-between">
    <div class="flex gap-1">
      <KunTooltip :text="`浏览量: ${galgame.view}`">
        <KunChip size="md">
          <KunIcon name="lucide:eye" />
          <span>{{ formatNumber(galgame.view) }}</span>
        </KunChip>
      </KunTooltip>

      <GalgameLike
        :galgame-id="galgame.id"
        :target-user-id="galgame.user.id"
        :like-count="galgame.like_count"
        :is-liked="galgame.is_liked"
      />

      <GalgameFavorite
        :galgame-id="galgame.id"
        :target-user-id="galgame.user.id"
        :favorite-count="galgame.favorite_count"
        :is-favorited="galgame.is_favorited"
      />

      <!-- E3b: the legacy rewrite entry retired — editing lives on the
           engine-backed /galgame/:gid/edit page (header button). -->
      <KunButton
        variant="light"
        color="default"
        size="sm"
        @click="navigateTo(`/galgame/${galgame.id}/edit`)"
      >
        <span class="flex items-center gap-1">
          <KunIcon name="lucide:file-pen-line" />编辑资料
        </span>
      </KunButton>
    </div>
  </div>
</template>
