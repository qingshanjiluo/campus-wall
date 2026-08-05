<script setup lang="ts">
// Compact list view for the /galgame-quiz browse page. Left = the viewer's own
// status (对/错/出题/未答); middle = title + author/time + 题型/分类; right =
// 难度 + 正确率.
import {
  KUN_QUIZ_TYPE_MAP,
  KUN_QUIZ_TYPE_ICON_MAP,
  KUN_QUIZ_CATEGORY_MAP,
  KUN_QUIZ_SPOILER_MAP,
  KUN_QUIZ_SPOILER_COLOR_MAP,
  kunQuizDifficultyLabel,
  kunQuizDifficultyColor
} from '~/constants/galgame-quiz'

defineProps<{ quizzes: GalgameQuizCard[] }>()

const correctRate = (q: GalgameQuizCard) =>
  q.answer_count > 0
    ? Math.round((q.correct_count / q.answer_count) * 100)
    : null
</script>

<template>
  <div class="divide-default-200 divide-y">
    <NuxtLink
      v-for="quiz in quizzes"
      :key="quiz.id"
      :to="`/galgame-quiz/${quiz.id}`"
      class="hover:bg-default-100 flex items-center gap-3 px-2 py-3 transition-colors"
    >
      <!-- left: the viewer's own status -->
      <div class="flex w-6 shrink-0 justify-center text-lg">
        <KunIcon
          v-if="quiz.my_status === 'correct'"
          name="lucide:circle-check"
          class="text-success"
        />
        <KunIcon
          v-else-if="quiz.my_status === 'incorrect'"
          name="lucide:circle-x"
          class="text-danger"
        />
        <KunIcon
          v-else-if="quiz.my_status === 'author'"
          name="lucide:pen-line"
          class="text-primary"
        />
        <KunIcon
          v-else-if="quiz.my_status === 'answered'"
          name="lucide:circle-check"
          class="text-default-400"
        />
        <KunIcon v-else name="lucide:circle-dashed" class="text-default-400" />
      </div>

      <!-- middle: title + meta -->
      <div class="min-w-0 flex-1">
        <p class="font-medium break-words">{{ quiz.question }}</p>
        <div
          class="text-default-500 mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs"
        >
          <span class="text-default-700 flex items-center gap-1">
            <KunAvatar
              :disable-floating="true"
              :user="quiz.user"
              size="xs"
              :is-navigation="false"
            />
            {{ quiz.user.name }}
          </span>
          <KunTime :time="quiz.status_update_time" />
          <span class="flex items-center gap-1">
            <KunIcon :name="KUN_QUIZ_TYPE_ICON_MAP[quiz.type]" />
            {{ KUN_QUIZ_TYPE_MAP[quiz.type] }}
          </span>
          <span>{{ KUN_QUIZ_CATEGORY_MAP[quiz.category] }}</span>
          <KunChip
            v-if="quiz.spoiler_level !== 'none'"
            :color="KUN_QUIZ_SPOILER_COLOR_MAP[quiz.spoiler_level]"
            variant="flat"
            size="sm"
          >
            {{ KUN_QUIZ_SPOILER_MAP[quiz.spoiler_level] }}
          </KunChip>
          <span class="flex items-center gap-1">
            <KunIcon name="lucide:eye" />{{ quiz.view }}
          </span>
          <span class="flex items-center gap-1">
            <KunIcon name="lucide:users" />{{ quiz.answer_count }}
          </span>
          <span v-if="quiz.favorite_count" class="flex items-center gap-1">
            <KunIcon name="lucide:heart" />{{ quiz.favorite_count }}
          </span>
          <span v-if="quiz.comment_count" class="flex items-center gap-1">
            <KunIcon name="lucide:message-square" />{{ quiz.comment_count }}
          </span>
        </div>
      </div>

      <!-- right: difficulty + correct rate -->
      <div class="flex shrink-0 flex-col items-end gap-1">
        <KunChip
          :color="kunQuizDifficultyColor(quiz.difficulty)"
          variant="flat"
          size="sm"
        >
          {{ kunQuizDifficultyLabel(quiz.difficulty) }} {{ quiz.difficulty }}
        </KunChip>
        <span
          v-if="correctRate(quiz) !== null"
          class="text-default-500 text-xs"
        >
          正确率 {{ correctRate(quiz) }}%
        </span>
      </div>
    </NuxtLink>
  </div>
</template>
