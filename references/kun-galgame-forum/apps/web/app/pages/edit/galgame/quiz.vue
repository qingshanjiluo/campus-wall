<script setup lang="ts">
definePageMeta({
  middleware: ['auth']
})

useKunDisableSeo('发布 Galgame 习题')

const onPublished = (quiz: GalgameQuizCard) =>
  navigateTo(`/galgame-quiz/${quiz.id}`)
const onCancel = () => navigateTo('/galgame-quiz')
</script>

<template>
  <div class="mx-auto max-w-3xl">
    <!-- Client-only: the form embeds the Milkdown editor (no SSR) and restores a
         localStorage draft, both of which would otherwise mismatch on hydrate. -->
    <ClientOnly>
      <GalgameQuizForm @published="onPublished" @cancel="onCancel" />
      <template #fallback>
        <div class="flex justify-center py-16">
          <KunLoading />
        </div>
      </template>
    </ClientOnly>
  </div>
</template>
