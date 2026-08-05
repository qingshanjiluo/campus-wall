<script setup lang="ts">
defineProps<{
  comment: SearchResultComment
}>()
</script>

<template>
  <KunLink
    color="default"
    underline="none"
    :to="commentPermalink(`/topic/${comment.topic_id}`, comment.id)"
    class="flex-col items-start"
  >
    <div class="flex items-center gap-2">
      <KunIcon class="text-primary h-5 w-5" name="uil:comment-dots" />
      <span class="text-lg">{{ comment.topic_title }}</span>
      <span class="text-default-500 ml-auto text-sm">
        <KunTime :time="comment.created" />
      </span>
    </div>

    <div
      class="border-primary bg-primary/10 my-2 rounded border-l-3 p-2 text-sm"
    >
      {{ comment.content }}
    </div>

    <div class="flex flex-wrap items-center gap-2">
      <div class="flex items-center">
        <KunAvatar :user="comment.user" :is-navigation="false" />
        <span class="ml-2 text-sm">{{ comment.user.name }}</span>
      </div>
      <!--
        BE `CommentItem` (search/dto) doesn't carry `target_user`. Guard
        with v-if so the arrow + avatar pair appears only when the
        chain parent is populated. See SearchResultComment in
        shared/types/search.ts for the optional-typed field.
      -->
      <template v-if="comment.target_user">
        <KunIcon name="lucide:arrow-right" class="h-4 w-4" />
        <div class="flex items-center">
          <KunAvatar :user="comment.target_user" :is-navigation="false" />
          <span class="ml-2 text-sm">{{ comment.target_user.name }}</span>
        </div>
      </template>
    </div>
  </KunLink>
</template>
