<script setup lang="ts">
import { KUN_DLSITE_ANNOUNCE_TOPIC_ID } from '~/constants/dlsite'

// The 补票 (buy-legit) prompt shown wherever a download resource is surfaced —
// the download modal and the resource detail panel. It used to be inlined in both
// with slightly different wording; one component keeps the ask consistent.
//
// TWO MODES, and the difference is the whole point:
//
//   - No DLsite id → the plain appeal. Nothing actionable to offer, so it stays
//     one quiet sentence pointing at the 制作商 section.
//   - DLsite id → the partnership offer. kungal is a DLsite affiliate that keeps
//     NONE of the revenue share. Part of it reaches buyers directly (the periodic
//     ¥400 coupon, the monthly ¥1000 coupon pool); the 12% point rebate cannot —
//     DLsite has no way to convert a share into per-user discounts — so it pools
//     in kungal's official DLsite account and buys games for the community.
//
// The offer mode deliberately leads with the benefit rather than the old guilt
// framing ("厂商不易, 请支持正版"). Two reasons. Buying through this link is now
// materially cheaper than going to DLsite directly, so the honest pitch is the
// discount, not a lecture. And an affiliate button sitting on a download page
// reads as the site monetising piracy traffic unless it says outright that the
// site takes nothing — which is true here, and is the most trust-building fact in
// the whole deal, so it gets its own line instead of being buried in a topic.
//
// Both URLs are assembled server-side — never build them here, the affiliate
// template lives in server config (this project's frontend build cannot be
// trusted with env vars).
const props = defineProps<{
  galgameId: number
  // Absent when the galgame has no DLsite id or the affiliate is unconfigured.
  purchaseUrl?: string
  // The partnership coupon page. Absent until it is configured (it must be a
  // shortened URL — the partner's anti-censorship requirement).
  couponUrl?: string
}>()

const hasOffer = computed(() => Boolean(props.purchaseUrl))

// The partnership terms, as scannable chips.
//
// The coupons come FIRST because they are the only part a buyer personally
// receives. The 12% is deliberately worded as going to the forum's shared
// account: DLsite cannot convert a revenue share into per-user discounts (the
// technical limit that shaped this whole deal), so those points land in kungal's
// official DLsite account and are spent on games for everyone — a buyer will
// never see them in their own account. Writing "购买返还 12% 点数" implies a
// personal rebate and would leave every buyer looking for points that are not
// there, on the one partnership whose entire premise is being straight about
// where the money goes.
const perks = [
  { icon: 'lucide:ticket', label: '每 3 个月 ¥400 优惠券' },
  { icon: 'lucide:gift', label: '每月 20 张 ¥1000 优惠券' },
  { icon: 'lucide:coins', label: '12% 返还点数 → 买游戏分享给大家' }
]
</script>

<template>
  <!-- Offer mode: a positive, actionable block. Deliberately NOT `danger` —
       red would read as a warning and compete with the real ones on this page
       (expired link / NSFW), and there is nothing to warn about here. -->
  <KunInfo
    v-if="hasOffer"
    color="primary"
    variant="bordered"
    title="支持正版 · 与 DLsite 官方合作"
  >
    <div class="space-y-3">
      <div class="flex flex-wrap gap-1.5">
        <KunChip
          v-for="perk in perks"
          :key="perk.label"
          color="primary"
          variant="flat"
          size="sm"
        >
          <span class="flex items-center gap-1">
            <KunIcon :name="perk.icon" />
            {{ perk.label }}
          </span>
        </KunChip>
      </div>

      <!-- The trust line. This is the fact that separates the partnership from
           ordinary affiliate spam, so it is stated plainly and not linked away.
           "用于回馈用户" rather than "返还给用户": part of it comes back as coupons
           a buyer personally uses, part as points in the forum's shared account
           spent on games for everyone — "返还" would promise a personal rebate. -->
      <p class="text-default-600 text-sm">
        通过本站链接购买, DLsite 给本站的
        <span class="text-default-800 font-medium">全部分成都用于回馈用户</span>
        —— 鲲 Galgame 不从这次合作中获得任何收益。
      </p>
      <p class="text-default-500 text-xs">
        DLsite 无法把分成折算成个人折扣, 因此这 12% 以点数形式汇入论坛公共账户,
        用于购买游戏并分享给所有人 —— 本站不留一分。
      </p>

      <div class="flex flex-wrap items-center gap-2">
        <KunButton
          :href="purchaseUrl"
          target="_blank"
          rel="noopener noreferrer"
          color="primary"
          size="sm"
          class-name="gap-1.5"
        >
          <KunIcon name="lucide:shopping-cart" />
          前往 DLsite 购买正版
        </KunButton>

        <KunButton
          v-if="couponUrl"
          :href="couponUrl"
          target="_blank"
          rel="noopener noreferrer"
          variant="light"
          color="primary"
          size="sm"
          class-name="gap-1.5"
        >
          <KunIcon name="lucide:ticket" />
          领取优惠券
        </KunButton>

        <KunLink
          v-if="KUN_DLSITE_ANNOUNCE_TOPIC_ID"
          size="sm"
          :to="`/topic/${KUN_DLSITE_ANNOUNCE_TOPIC_ID}`"
        >
          合作详情
        </KunLink>
      </div>
    </div>
  </KunInfo>

  <!-- Plain appeal: no DLsite id for this galgame, so there is nothing to offer. -->
  <KunInfo v-else color="danger" variant="bordered" title="补票提示">
    <p class="text-sm">
      Galgame 厂商制作游戏不易, 很多厂商如今都在炒冷饭, 可见经济并不宽裕。
      如果条件允许, 可前往
      <KunLink size="sm" :to="`/galgame/${galgameId}`" class-name="inline">
        Galgame 详情
      </KunLink>
      中的制作商部分进行正版补票, 感谢您对 Galgame 业界做出的贡献。
    </p>
  </KunInfo>
</template>
