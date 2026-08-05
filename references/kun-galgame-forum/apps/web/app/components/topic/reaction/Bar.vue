<script setup lang="ts">
// Reaction chips below a topic / reply: detail chips show up to MAX_AVATARS
// reactor avatars + a "+N" overflow (uniform height — every chip has avatars);
// feed chips (no reactors shipped) show emoji + count. Tap a chip to toggle.
// State is provided by the parent (useReactions) so these chips and the trigger
// button stay in sync even when the trigger lives elsewhere (e.g. the footer).
import { KUN_REACTION_EMOJI, reactionAsset } from '~/constants/reaction'

const { list, toggle } = inject(reactionsKey)!

const MAX_AVATARS = 3
const shownReactors = (r: KunReaction) => r.reactors?.slice(0, MAX_AVATARS) ?? []
// Users beyond the shown avatars → "+N"; 0 hides the badge.
const overflow = (r: KunReaction) => r.count - shownReactors(r).length
</script>

<template>
  <div v-if="list.length" class="flex flex-wrap items-center gap-1.5">
    <button
      v-for="r in list"
      :key="r.reaction"
      type="button"
      :class="
        cn(
          'flex items-center gap-1 rounded-full border px-2 py-0.5 text-sm transition-colors',
          r.mine
            ? 'border-primary bg-primary/10 text-primary'
            : 'border-default-200 text-default-600 hover:bg-default-100'
        )
      "
      @click="toggle(r.reaction)"
    >
      <img
        :src="reactionAsset(r.reaction)"
        :alt="KUN_REACTION_EMOJI[r.reaction] ?? r.reaction"
        class="size-5 shrink-0 max-w-none"
        loading="lazy"
      />
      <template v-if="r.reactors?.length">
        <span class="flex -space-x-1.5">
          <KunAvatar
            v-for="u in shownReactors(r)"
            :key="u.id"
            :user="u"
            size="sm"
            :is-navigation="false"
          />
        </span>
        <span v-if="overflow(r) > 0" class="tabular-nums">
          +{{ formatNumber(overflow(r)) }}
        </span>
      </template>
      <span v-else class="tabular-nums">{{ formatNumber(r.count) }}</span>
    </button>
  </div>
</template>
