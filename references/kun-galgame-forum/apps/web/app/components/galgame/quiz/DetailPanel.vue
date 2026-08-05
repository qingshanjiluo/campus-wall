<script setup lang="ts">
// 查看题目详情 on the answer page (/galgame-quiz/:id). The FULL detail (linked
// game + correct/wrong chart + answerer records) is always rendered; collapsed
// it's clipped to a peek and obscured by a bottom fade, and 查看详情 just
// removes the clip. Records are fetched once up-front so the content is complete.
const props = defineProps<{ quiz: GalgameQuizPlay }>()

const expanded = ref(false)
const bodyRef = ref<HTMLElement | null>(null)
// 查看详情 expand/collapse. The panel is always rendered but clipped to a 6rem
// peek; expanding animates max-height to the content's REAL height and then
// releases the cap (max-height:none) on transitionend, so tall detail (many
// linked games + a long records list) is never clipped and the page scrolls past
// it. Collapsing pins the height, then animates back down to the peek. (The old
// fixed 48rem cap clipped taller quizzes with no scroll.)
const maxHeight = ref('6rem')
const expand = () => {
  const el = bodyRef.value
  maxHeight.value = el ? `${el.scrollHeight}px` : 'none'
  expanded.value = true
}
const collapse = () => {
  const el = bodyRef.value
  if (el) maxHeight.value = `${el.scrollHeight}px`
  expanded.value = false
  requestAnimationFrame(() =>
    requestAnimationFrame(() => {
      maxHeight.value = '6rem'
    })
  )
}
const onTransitionEnd = (e: TransitionEvent) => {
  if (e.propertyName === 'max-height' && expanded.value) maxHeight.value = 'none'
}
const recordsData = ref<QuizAnswererRecord[] | null>(null)
const loading = ref(false)

const wrong = computed(() => props.quiz.answer_count - props.quiz.correct_count)

// Compact one-line rendering of an answerer's own submission. Only ever shown
// when `rec.submitted` is present — the server omits it for viewers who haven't
// answered, so it can't leak the correct answer. Single/multiple show the chosen
// option letter(s) (the viewer saw the options, so A/B/C is meaningful); judge /
// fill / essay are self-contained.
const letter = (i: number) => String.fromCharCode(65 + i)
const formatSubmitted = (s: QuizSubmitted): string => {
  switch (props.quiz.type) {
    case 'single': {
      const v = (s as { value: number }).value
      return typeof v === 'number' ? `选 ${letter(v)}` : ''
    }
    case 'multiple': {
      const vals = (s as { values: number[] }).values ?? []
      return vals.length ? `选 ${vals.map(letter).join('、')}` : '未选'
    }
    case 'judge':
      return (s as { value: boolean }).value ? '答: 正确' : '答: 错误'
    case 'fill': {
      const vals = (s as { values: string[] }).values ?? []
      return `填: ${vals.map((v) => v || '(空)').join(' / ')}`
    }
    case 'essay':
      return `答: ${(s as { text: string }).text || '(空)'}`
    default:
      return ''
  }
}

onMounted(async () => {
  loading.value = true
  const data = await kunFetch<QuizAnswererRecord[]>(
    `/galgame-quiz/${props.quiz.id}/answers`
  )
  recordsData.value = data ?? []
  loading.value = false
})
</script>

<template>
  <div class="space-y-1">
    <div class="relative">
      <div
        ref="bodyRef"
        class="overflow-hidden transition-[max-height] duration-300"
        :style="{ maxHeight }"
        @transitionend="onTransitionEnd"
      >
        <div class="space-y-4">
          <!-- linked galgames (or a "revealed after answering" hint) -->
          <div
            v-for="g in quiz.galgames"
            :key="g.id"
            class="border-default-200 flex gap-3 rounded-lg border p-2"
          >
            <div
              class="bg-default-100 h-20 w-32 shrink-0 overflow-hidden rounded-lg"
            >
              <KunImage
                v-if="g.banner"
                :src="g.banner"
                :thumbhash="g.banner_thumbhash"
                width="128"
                height="80"
                object-fit="cover"
                class-name="h-full w-full"
              />
            </div>
            <div class="min-w-0 flex-1">
              <KunLink :to="`/galgame/${g.id}`" class="font-medium">
                {{ getPreferredLanguageText(g.name) }}
              </KunLink>
              <div class="mt-1 flex flex-wrap items-center gap-1">
                <KunChip v-if="g.age_limit" size="sm" variant="flat">
                  {{ g.age_limit }}
                </KunChip>
                <KunChip v-if="g.original_language" size="sm" variant="light">
                  {{ g.original_language }}
                </KunChip>
              </div>
              <p
                v-if="g.officials.length"
                class="text-default-500 mt-1 line-clamp-2 text-xs"
              >
                {{ g.officials.join('、') }}
              </p>
            </div>
          </div>
          <div
            v-if="!quiz.galgames.length && quiz.hide_galgame && !quiz.my_answer"
            class="text-default-500 border-default-200 flex items-center gap-2 rounded-lg border border-dashed p-3 text-sm"
          >
            <KunIcon name="lucide:lock" />
            关联作品已隐藏, 作答后揭晓
          </div>

          <!-- correct / wrong chart -->
          <div>
            <p class="text-default-400 mb-2 text-xs">作答统计</p>
            <div v-if="quiz.answer_count > 0" class="flex items-center gap-4">
              <KunProgress
                variant="circle"
                :value="quiz.correct_count"
                :max="quiz.answer_count"
                color="success"
                size="lg"
                show-label
              />
              <div class="space-y-1 text-sm">
                <p class="text-success flex items-center gap-1">
                  <KunIcon name="lucide:check" />正确 {{ quiz.correct_count }}
                </p>
                <p class="text-danger flex items-center gap-1">
                  <KunIcon name="lucide:x" />错误 {{ wrong }}
                </p>
                <p class="text-default-500">共 {{ quiz.answer_count }} 人作答</p>
                <p v-if="quiz.quality_count > 0" class="text-default-500">
                  质量 {{ quiz.quality_average }} ({{ quiz.quality_count }} 人)
                </p>
              </div>
            </div>
            <p v-else class="text-default-500 text-sm">暂无作答</p>
          </div>

          <!-- answerer records -->
          <div>
            <p class="text-default-400 mb-1 text-xs">作答记录</p>
            <p v-if="loading" class="text-default-500 text-sm">加载中…</p>
            <p v-else-if="!recordsData?.length" class="text-default-500 text-sm">
              还没有人作答
            </p>
            <div v-else class="max-h-64 space-y-1 overflow-y-auto pr-1">
              <div
                v-for="(rec, i) in recordsData"
                :key="i"
                class="hover:bg-default-100 rounded-md px-1 py-1"
              >
                <div class="flex items-center gap-2 text-sm">
                  <KunAvatar
                    :disable-floating="true"
                    :user="rec.user"
                    size="xs"
                    :is-navigation="false"
                  />
                  <span class="text-default-700 min-w-0 flex-1 truncate">
                    {{ rec.user.name }}
                  </span>
                  <KunTime
                    :time="rec.created"
                    class="text-default-400 text-xs"
                  />
                  <KunChip
                    v-if="rec.is_correct === true"
                    color="success"
                    variant="flat"
                    size="sm"
                  >
                    正确
                  </KunChip>
                  <KunChip
                    v-else-if="rec.is_correct === false"
                    color="danger"
                    variant="flat"
                    size="sm"
                  >
                    错误
                  </KunChip>
                </div>
                <!-- Answer result, its own line — only present for a viewer who
                     has answered (the server omits `submitted` for non-answerers,
                     so it can never leak the correct answer). -->
                <p
                  v-if="rec.submitted"
                  class="text-default-500 mt-0.5 pl-7 text-xs break-words whitespace-pre-wrap"
                >
                  {{ formatSubmitted(rec.submitted) }}
                </p>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- bottom fade scrim (collapsed only). This transparent→background
           gradient is a SANCTIONED EXCEPTION to 铁律 #2 (no gradients),
           explicitly authorized for this "peek then reveal" affordance. -->
      <div
        v-show="!expanded"
        class="pointer-events-none absolute inset-x-0 bottom-0 h-10 bg-gradient-to-t from-[oklch(var(--content1))] to-transparent"
      />
    </div>

    <div class="flex justify-end">
      <KunButton
        variant="light"
        size="sm"
        @click="expanded ? collapse() : expand()"
      >
        <span class="flex items-center gap-1">
          <KunIcon
            :name="expanded ? 'lucide:chevron-up' : 'lucide:chevron-down'"
          />
          {{ expanded ? '收起' : '查看详情' }}
        </span>
      </KunButton>
    </div>
  </div>
</template>
