<script setup lang="ts">
import {
  KUN_GALGAME_AGE_LIMIT_MAP,
  getGalgameOriginalLanguageName
} from '~/constants/galgame'

defineProps<{
  official: GalgameOfficialItem[]
  engine: GalgameEngineItem[]
  series: GalgameDetailSeriesRef[]
  originalLanguage: string
  ageLimit: 'all' | 'r18'
  // U1: nil/'' = unknown release; TBA flag is independent. Rendering
  // logic in shared/utils/getReleaseDateText.ts.
  releaseDate?: string | null
  releaseDateTBA?: boolean
}>()

// E3b: the legacy 编辑历史/更新请求 modal retired — history lives on the
// engine-backed /galgame/:gid/history page. The detail object is provided
// at the page root (the link needs the gid).
const galgame = inject<GalgameDetail>('galgame')

const getLanguageName = getGalgameOriginalLanguageName
</script>

<template>
  <KunCard
    :is-hoverable="false"
    :is-transparent="false"
    class-name="overflow-visible"
    content-class="space-y-3"
  >
    <dl class="space-y-5">
      <GalgameDetailOfficial :official="official" />

      <!-- 系列: the grouping entity, linked because the question it answers
           ("还有哪几部") lives on the series page, not here. Identity only —
           the member count belongs to that page, and a second number here
           would eventually disagree with it. -->
      <div v-if="series && series.length > 0">
        <dt class="text-default-500 dark:text-default-400 text-sm font-medium">
          所属系列
        </dt>
        <dd
          class="text-default-900 mt-1.5 flex flex-wrap items-center gap-x-3 text-base font-medium dark:text-white"
        >
          <KunLink
            v-for="item in series"
            :key="item.id"
            :to="`/galgame/series/${item.id}`"
            underline="none"
            class-name="text-foreground hover:text-primary text-base font-semibold"
          >
            {{ item.name }}
          </KunLink>
        </dd>
      </div>

      <div
        v-if="engine && engine.length > 0"
        class="border-default-200 dark:border-default-700/50"
      >
        <dt class="text-default-500 dark:text-default-400 text-sm font-medium">
          游戏引擎
        </dt>
        <dd
          class="text-default-900 mt-1.5 flex flex-wrap items-center gap-x-3 text-base font-medium dark:text-white"
        >
          <KunLink
            v-for="item in engine"
            :key="item.id"
            :to="`/galgame/engine/${item.id}`"
            underline="none"
            class-name="text-foreground hover:text-primary text-base font-semibold"
          >
            {{ item.name }}
            <!-- No count (upstream not yet publishing one, or a genuinely
                 unused engine) ⇒ no badge and no tooltip, never "+ 0". -->
            <KunTooltip
              v-if="item.galgame_count > 0"
              :text="`${item.galgame_count} 个 Galgame 使用此引擎制作`"
            >
              <KunChip size="xs">
                {{ `+ ${item.galgame_count}` }}
              </KunChip>
            </KunTooltip>
          </KunLink>
        </dd>
      </div>

      <div class="flex items-center justify-between">
        <dt class="text-default-500 text-sm font-medium">游戏原语言</dt>
        <dd>
          <KunChip color="warning">
            {{ getLanguageName(originalLanguage) }}
          </KunChip>
        </dd>
      </div>

      <div class="flex items-center justify-between">
        <dt class="text-default-500 text-sm font-medium">发售日期</dt>
        <dd class="text-default-700 text-sm">
          {{ getReleaseDateText(releaseDate, releaseDateTBA) }}
        </dd>
      </div>

      <div class="flex items-center justify-between">
        <dt class="text-default-500 dark:text-default-400 text-sm font-medium">
          年龄限制
        </dt>
        <dd>
          <KunTooltip
            position="left"
            :text="KUN_GALGAME_AGE_LIMIT_MAP[ageLimit]"
          >
            <KunChip
              variant="flat"
              :color="ageLimit === 'all' ? 'success' : 'danger'"
            >
              {{ ageLimit === 'all' ? '全年龄' : 'R18' }}
            </KunChip>
          </KunTooltip>
        </dd>
      </div>
    </dl>

    <!-- The engine-backed revision history page (includes the migrated
         pre-engine history + any-two-versions diff). -->
    <KunButton
      v-if="galgame"
      variant="flat"
      color="primary"
      size="sm"
      full-width
      @click="navigateTo(`/galgame/${galgame.id}/history`)"
    >
      <KunIcon name="lucide:history" />
      修订历史
    </KunButton>
  </KunCard>
</template>
