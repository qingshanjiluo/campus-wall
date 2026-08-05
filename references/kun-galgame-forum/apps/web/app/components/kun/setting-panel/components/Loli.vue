<script setup lang="ts">
import { getLoli } from './getLoli'

// ── Containing the 差分图 ──────────────────────────────────────────────
// The layer JSON (public/ren.json) places every part on the ORIGINAL game
// canvas. Across all selectable parts the assembled character occupies the
// rectangle (137,323)→(504,925) — i.e. a 367×602 portrait whose top-left sits
// at (137,323); the body layer is the full silhouette, the eyes/brow/mouth are
// small overlays on the head. The old code just shoved that 600px-tall portrait
// up-left with negative offsets so a slice peeked into the panel, which is why
// it never "fit".
//
// Fixed stage window (canvas units) = the union extent, scaled to a panel-
// friendly height (~421px). Each character is CENTERED in this window on its
// OWN bbox (getLoli computes it), so narrower skirts no longer sit shoved to
// one side. CANVAS spans the parts' full coordinate space so the absolutely-
// positioned imgs get a real containing block (not crushed by a global max-width).
const FRAME_W = 367
const FRAME_H = 602
const CANVAS_W = 504
const CANVAS_H = 925
const SCALE = 0.7
const stageW = Math.round(FRAME_W * SCALE)
const stageH = Math.round(FRAME_H * SCALE)

const loliData = ref({
  loliBodyLeft: '',
  loliBodyTop: '',
  loliEyeLeft: '',
  loliEyeTop: '',
  loliBrowLeft: '',
  loliBrowTop: '',
  loliMouthLeft: '',
  loliMouthTop: '',
  loliFaceLeft: '',
  loliFaceTop: '',
  body: '',
  eye: '',
  brow: '',
  mouth: '',
  face: '',
  bbox: { left: 137, top: 323, width: 367, height: 602 }
})

// Center the assembled character's bbox in the fixed FRAME window.
const canvasStyle = computed(() => {
  const b = loliData.value.bbox
  const x0 = b.left + b.width / 2 - FRAME_W / 2
  const y0 = b.top + b.height / 2 - FRAME_H / 2
  return {
    width: `${CANVAS_W}px`,
    height: `${CANVAS_H}px`,
    transform: `scale(${SCALE}) translate(${-x0}px, ${-y0}px)`,
    transformOrigin: 'top left'
  }
})

// The 5 webp layers are fetched + blob-encoded on demand. We DECODE all of them
// before revealing, so head/eyes/brows/mouth/body paint together (no part-by-part
// pop-in) — and nothing (no KunLoading) shows until they're ready. `ready` flips
// true after the first successful load and stays true; a re-roll keeps the current
// character on screen and swaps atomically once the new one is fully decoded, so
// there's never a blank flash.
const ready = ref(false)

const decode = (src: string) => {
  if (!src) return Promise.resolve()
  const img = new Image()
  img.src = src
  return img.decode().catch(() => {})
}

const reroll = async () => {
  const data = await getLoli()
  await Promise.all(
    [data.body, data.eye, data.brow, data.mouth, data.face].map(decode)
  )
  loliData.value = data
  ready.value = true
}

onMounted(reroll)
</script>

<template>
  <div
    class="hidden shrink-0 sm:block"
    :style="{ width: `${stageW}px`, height: `${stageH}px` }"
  >
    <!-- Nothing until every layer has decoded; then all parts paint at once. -->
    <KunTooltip v-if="ready && loliData.body" text="点击换一个孩子" position="left">
      <div
        class="relative cursor-pointer overflow-hidden"
        :style="{ width: `${stageW}px`, height: `${stageH}px` }"
        @click="reroll"
      >
        <div class="absolute top-0 left-0" :style="canvasStyle">
          <img
            class="absolute max-w-none"
            :src="loliData.body"
            alt="ren"
            :style="{ left: loliData.loliBodyLeft, top: loliData.loliBodyTop }"
          />
          <img
            class="absolute max-w-none"
            :src="loliData.eye"
            alt="ren"
            :style="{ left: loliData.loliEyeLeft, top: loliData.loliEyeTop }"
          />
          <img
            class="absolute max-w-none"
            :src="loliData.brow"
            alt="ren"
            :style="{ left: loliData.loliBrowLeft, top: loliData.loliBrowTop }"
          />
          <img
            class="absolute max-w-none"
            :src="loliData.mouth"
            alt="ren"
            :style="{
              left: loliData.loliMouthLeft,
              top: loliData.loliMouthTop
            }"
          />
          <img
            class="absolute max-w-none"
            :src="loliData.face"
            alt="ren"
            :style="{ left: loliData.loliFaceLeft, top: loliData.loliFaceTop }"
          />
        </div>
      </div>
    </KunTooltip>
  </div>
</template>
