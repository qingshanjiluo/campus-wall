<script setup lang="ts">
import { kunGalgameOriginalLanguageOptions } from '~/constants/galgame'

// Wiki accepts these fields on POST /galgame; defaults (ja-jp / r18) are
// fine for most Japanese visual novels but we expose them as choices so
// authors can correct non-default games (English VNs, all-ages titles) at
// publish time instead of needing a follow-up PR. See audit batch 2 §10.
//
// U1 (release date): wiki stores a real date + TBA flag instead of a
// free-form string. Empty input = unknown; TBA toggle is independent of
// the date (a TBA-flagged entry can still carry a predicted date).

const { age_limit, original_language, release_date, release_date_tba } = storeToRefs(
  usePersistEditGalgameStore()
)

const ageLimitOptions = [
  { value: 'all', label: '全年龄 (本游戏不含成人内容)' },
  { value: 'r18', label: 'R18 (本游戏含成人内容)' }
] as const

// Full set the wiki actually uses (Korean / Russian / French / …), shared from
// constants so display + dropdown + validation stay in sync. A claimed draft in
// any of these now round-trips on re-edit instead of failing the old 4-value
// enum and forcing a manual re-pick.
const originalLanguageOptions = kunGalgameOriginalLanguageOptions
</script>

<template>
  <div class="space-y-2">
    <h2 class="text-xl">游戏基础信息</h2>
    <p class="text-default-500 text-sm">
      默认假设是日语原版、含成人内容的视觉小说。如果游戏与默认不同，请在此调整。
    </p>
    <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
      <KunSelect
        v-model="age_limit"
        label="年龄分级"
        :options="ageLimitOptions"
      />
      <KunSelect
        v-model="original_language"
        label="原始语言"
        :options="originalLanguageOptions"
      />
    </div>
    <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
      <KunInput
        v-model="release_date"
        type="date"
        label="发售日期 (可留空)"
      />
      <KunSwitch v-model="release_date_tba" label="发售日期待定 (TBA)" />
    </div>
  </div>
</template>
