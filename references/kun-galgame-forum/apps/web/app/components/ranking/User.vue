<script setup lang="ts">
import { userSortItem } from '~/constants/ranking'
import { userRankingPageData, getRankClasses } from './pageData'

const { data } = await useKunFetch<RankingUserItem[]>('/ranking/user', {
  query: userRankingPageData
})
</script>

<template>
  <ul v-if="data" class="space-y-3">
    <li v-for="(user, index) in data" :key="user.id">
      <KunLink
        color="default"
        underline="none"
        :to="`/user/${user.id}`"
        :class-name="
          cn(
            'relative flex items-center gap-3 rounded-xl border p-3 transition-colors',
            getRankClasses(index)
          )
        "
      >
        <RankingMedal :index="index" />

        <KunAvatar :user="user" size="lg" :is-navigation="false" />
        <div class="flex-1 overflow-hidden">
          <h3 class="flex items-center gap-2 font-semibold">
            <span>{{ user.name }}</span>
            <div class="flex shrink-0 items-center gap-2 sm:hidden">
              <KunIcon
                :name="
                  userSortItem.find((i) => i.sortField === user.sort_field)
                    ?.icon || ''
                "
                class="text-primary h-5 w-5"
              />
              <span class="text-default-500 text-sm font-normal">
                {{ user.value }}
              </span>
            </div>
          </h3>
          <p class="text-default-500 truncate text-sm">
            {{ user.bio || '这只笨蛋萝莉居然不写签名! 八嘎!' }}
          </p>
        </div>

        <div class="hidden shrink-0 items-center gap-2 sm:flex">
          <KunIcon
            :name="
              userSortItem.find((i) => i.sortField === user.sort_field)?.icon ||
              ''
            "
            class="text-primary h-5 w-5"
          />
          <span class="text-lg font-medium">{{ user.value }}</span>
        </div>
      </KunLink>
    </li>
  </ul>
</template>
