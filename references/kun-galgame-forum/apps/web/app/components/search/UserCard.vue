<script setup lang="ts">
defineProps<{
  user: SearchResultUser
}>()
</script>

<template>
  <KunCard :href="`/user/${user.id}`">
    <div class="flex items-center">
      <KunAvatar :disable-floating="true" :user="user" :is-navigation="false" />
      <span class="ml-2">{{ user.name }}</span>
    </div>

    <pre v-if="user.bio" class="mt-2 text-sm break-all whitespace-pre-wrap">
      {{ user.bio }}
    </pre>

    <!--
      BE `UserItem` (search/dto) currently leaves `moemoepoint`+`created`
      zero (kungal_user_state not joined at search time). Guard each
      block so we don't render "0 萌点" / "Date(zero)". Drop the whole
      row when neither is available.
    -->
    <div
      v-if="user.moemoepoint || user.created"
      class="mt-2 flex items-center justify-between text-sm"
    >
      <div
        v-if="user.moemoepoint"
        class="text-secondary flex items-center"
      >
        <KunIcon name="lucide:lollipop" class="h-5 w-5" />
        {{ user.moemoepoint }}
      </div>
      <span v-if="user.created" class="text-default-700">
        <KunTime :time="user.created" type="date" show-year />
      </span>
    </div>
  </KunCard>
</template>
