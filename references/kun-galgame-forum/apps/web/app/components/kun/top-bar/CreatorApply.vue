<script setup lang="ts">
// Creator-application modal. Mounted at the app.vue root (a stable, non-scoped
// node) — NOT inside the avatar popover, which v-if-unmounts on click-away and
// would tear this modal down with it (same reasoning as 退出登录 / 萌萌点明细).
// UserInfo's "创作者申请" sets the temp-store flag; this self-binds to it.
//
// Replaces the old buried settings card: it surfaces the benefits + the live
// eligibility the moment it opens, and lets an eligible user apply in one click.
interface CreatorEligibility {
  eligible: boolean
  merged_prs: number
  galgames_published: number
  reviews_100: number
  moemoepoint: number
  need_merged_prs: number
  need_galgames: number
  need_reviews: number
  need_moemoepoint: number
}
interface CreatorApplicationInfo {
  id: number
  status: string
  decline_reason: string
}
interface CreatorStatus {
  eligibility: CreatorEligibility
  application: CreatorApplicationInfo | null
  is_creator: boolean
}

const { showKUNGalgameCreatorApply: isOpen } = storeToRefs(
  useTempSettingStore()
)

const status = ref<CreatorStatus | null>(null)
const message = ref('')
const loading = ref(false)
const failed = ref(false)
const submitting = ref(false)

const load = async () => {
  loading.value = true
  failed.value = false
  const res = await kunFetch<CreatorStatus>('/user/creator/status')
  if (res) {
    status.value = res
  } else {
    failed.value = true
  }
  loading.value = false
}

// Re-fetch on every open so eligibility is fresh; clear any stale draft.
watch(isOpen, (open) => {
  if (open) {
    message.value = ''
    load()
  }
})

const eligibility = computed(() => status.value?.eligibility ?? null)
const application = computed(() => status.value?.application ?? null)
const isCreator = computed(
  () => !!status.value?.is_creator || application.value?.status === 'approved'
)
const isPending = computed(() => application.value?.status === 'pending')
const isDeclined = computed(() => application.value?.status === 'declined')
const isEligible = computed(() => !!eligibility.value?.eligible)
const canApply = computed(
  () => isEligible.value && !isPending.value && !isCreator.value
)

// Application flow as a KunSteps stepper. current is 0-based; earlier steps
// render done (check). 0 达成条件 · 1 提交申请 · 2 审核 · 3 成为创作者.
const FLOW = [
  { title: '达成条件', icon: 'lucide:target' },
  { title: '提交申请', icon: 'lucide:send' },
  { title: '管理员审核', icon: 'lucide:user-round-check' },
  { title: '成为创作者', icon: 'lucide:party-popper' }
]
const currentStep = computed(() => {
  if (isCreator.value) return 3
  if (isPending.value) return 2
  if (isEligible.value) return 1
  return 0
})

const BENEFITS = [
  '直接发布 Galgame 词条，无需排队等待审核',
  '收录 VNDB 未登录的原创 / 同人 / 独立作品',
  '提交即时生效，编辑已发布条目更自由'
]

const conditions = computed(() => {
  const e = eligibility.value
  if (!e) return []
  return [
    {
      label: '已经被合并的 Galgame 信息更新请求',
      cur: e.merged_prs,
      need: e.need_merged_prs
    },
    {
      label: '已发布 Galgame',
      cur: e.galgames_published,
      need: e.need_galgames
    },
    { label: '百字以上简评', cur: e.reviews_100, need: e.need_reviews },
    { label: '萌萌点', cur: e.moemoepoint, need: e.need_moemoepoint }
  ].map((c) => ({
    ...c,
    met: c.cur >= c.need,
    pct: c.need > 0 ? Math.min(100, Math.round((c.cur / c.need) * 100)) : 100
  }))
})

const handleApply = async () => {
  if (!canApply.value) return
  submitting.value = true
  const res = await kunFetch<CreatorApplicationInfo>('/user/creator/apply', {
    method: 'POST',
    body: { message: message.value }
  })
  submitting.value = false
  if (res) {
    useMessage('申请已提交，等待管理员审核', 'success')
    await load()
  }
}
</script>

<template>
  <KunModal v-model="isOpen" inner-class-name="max-w-xl">
    <div class="space-y-5 p-1">
      <!-- header -->
      <div class="flex items-start gap-3">
        <KunIcon
          class="text-primary mt-0.5 size-8 shrink-0"
          name="lucide:badge-check"
        />
        <div class="space-y-0.5">
          <h2 class="text-foreground text-lg font-semibold">成为创作者</h2>
          <p class="text-default-500 text-sm">
            创作者是社区里值得信赖的发布者，可直接为大家收录 Galgame 词条。
          </p>
        </div>
      </div>

      <div v-if="loading" class="flex justify-center py-12">
        <KunIcon
          class="text-primary size-7 animate-spin"
          name="lucide:loader-circle"
        />
      </div>

      <div v-else-if="failed" class="flex flex-col items-center gap-3 py-10">
        <p class="text-default-500 text-sm">加载失败</p>
        <KunButton variant="flat" size="sm" @click="load">重试</KunButton>
      </div>

      <template v-else-if="status">
        <KunSteps
          :items="FLOW"
          :current="currentStep"
          color="primary"
          size="sm"
        />

        <!-- already a creator (edge: just approved before the role cache caught up) -->
        <div
          v-if="isCreator"
          class="bg-success-50 text-success-700 flex items-center gap-2 rounded-xl p-4 text-sm"
        >
          <KunIcon class="size-5 shrink-0" name="lucide:party-popper" />
          你已是创作者，可直接发布 Galgame 词条，无需再次申请。
        </div>

        <template v-else>
          <!-- benefits -->
          <section class="space-y-2">
            <h3 class="text-default-700 text-sm font-medium">创作者特权</h3>
            <ul class="text-default-700 list-disc space-y-1.5 pl-5 text-sm">
              <li v-for="b in BENEFITS" :key="b">{{ b }}</li>
            </ul>
          </section>

          <KunDivider />

          <!-- conditions -->
          <section class="space-y-3">
            <div class="flex items-center justify-between">
              <h3 class="text-default-700 text-sm font-medium">申请条件</h3>
              <KunChip
                size="sm"
                variant="flat"
                :color="isEligible ? 'success' : 'default'"
              >
                满足任一即可 · {{ isEligible ? '已满足' : '未满足' }}
              </KunChip>
            </div>
            <div v-for="c in conditions" :key="c.label" class="space-y-1">
              <div class="flex items-center justify-between gap-2 text-sm">
                <span>{{ c.label }}</span>
                <span
                  class="shrink-0"
                  :class="
                    c.met ? 'text-success font-medium' : 'text-default-500'
                  "
                >
                  {{ c.cur }} / {{ c.need }}
                </span>
              </div>
              <KunProgress
                :value="c.pct"
                size="sm"
                :color="c.met ? 'success' : 'primary'"
              />
            </div>
          </section>

          <!-- current application state -->
          <div
            v-if="isPending"
            class="bg-primary-50 text-primary-700 flex items-center gap-2 rounded-xl p-3 text-sm"
          >
            <KunIcon class="size-5 shrink-0" name="lucide:clock" />
            申请审核中，管理员会尽快处理，结果将通过站内消息通知你。
          </div>
          <div
            v-else-if="isDeclined"
            class="bg-warning-50 text-warning-700 space-y-1 rounded-xl p-3 text-sm"
          >
            <div class="flex items-center gap-2 font-medium">
              <KunIcon class="size-5 shrink-0" name="lucide:circle-x" />
              上次申请未通过
            </div>
            <p v-if="application?.decline_reason" class="text-warning-600 pl-7">
              原因：{{ application.decline_reason }}
            </p>
          </div>

          <!-- optional message, only when actually applying -->
          <KunTextarea
            v-if="canApply"
            name="creator-message"
            placeholder="(可选) 附言：向管理员说明你的情况"
            :rows="2"
            v-model="message"
          />

          <!-- CTA -->
          <div class="flex items-center justify-end gap-2 pt-1">
            <KunButton variant="light" @click="isOpen = false">
              稍后再说
            </KunButton>
            <KunButton
              v-if="!isPending"
              color="primary"
              :disabled="!canApply"
              :loading="submitting"
              @click="handleApply"
            >
              {{
                canApply ? (isDeclined ? '重新申请' : '立即申请') : '继续努力'
              }}
            </KunButton>
          </div>
        </template>
      </template>
    </div>
  </KunModal>
</template>
