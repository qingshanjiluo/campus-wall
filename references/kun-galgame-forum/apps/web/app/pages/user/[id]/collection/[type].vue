<script setup lang="ts">
import {
  kunUserCollectionNavItem,
  type KUN_USER_PAGE_COLLECTION_TYPE
} from '~/constants/user'

const props = defineProps<{
  user: UserInfo
}>()

const route = useRoute()
const collectionType = computed(() => {
  const t = (route.params as { type: string }).type
  return (t === 'topic' ? 'topic' : 'galgame') as
    (typeof KUN_USER_PAGE_COLLECTION_TYPE)[number]
})

useKunDisableSeo(`${props.user.name}的收藏`)
</script>

<template>
  <div class="space-y-3">
    <KunTab
      :items="kunUserCollectionNavItem(user.id)"
      :model-value="collectionType"
      variant="light"
      size="sm"
      scrollable
    />

    <UserCollectionGalgame
      v-if="collectionType === 'galgame'"
      :user-id="user.id"
      :owner-name="user.name"
    />
    <UserCollectionTopic v-else :user-id="user.id" />
  </div>
</template>
