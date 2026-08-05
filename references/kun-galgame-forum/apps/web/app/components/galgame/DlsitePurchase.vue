<script setup lang="ts">
import { KUN_DLSITE_ANNOUNCE_TOPIC_ID } from '~/constants/dlsite'

// The 正版购买 entry in the galgame header. Clicking it opens a popover with the
// two things a buyer can do, in the order they should do them: claim the coupon
// first, then open the product page. Presenting them as a plain pair of links
// would hide that ordering, and a user who buys before claiming loses the ¥400.
//
// The header has no room for the full pitch (the 补票 notice on the download
// surfaces carries that), so this stays a compact button + popover. Each row gets
// one explanatory line, because "领取优惠券" alone does not say how often, how
// much, or that there is a minimum spend.
//
// Degrades on purpose: with no coupon configured there is only one action left, so
// the trigger becomes a direct link rather than a popover wrapping a single item.
const props = defineProps<{
  purchaseUrl: string
  couponUrl?: string
}>()

const hasCoupon = computed(() => Boolean(props.couponUrl))
</script>

<template>
  <!-- Coupon configured → the two-step popover. -->
  <KunPopover v-if="hasCoupon" position="bottom-end">
    <template #trigger>
      <KunButton variant="flat" color="primary" size="sm">
        <span class="flex items-center gap-1">
          <KunIcon name="lucide:shopping-cart" />正版购买
        </span>
      </KunButton>
    </template>

    <div class="w-80 max-w-[88vw] p-2">
      <div class="px-2 pt-1 pb-2">
        <p class="text-default-800 text-sm font-medium">与 DLsite 官方合作</p>
        <!-- "用于回馈用户", not "返还给用户": the coupons reach a buyer directly,
             but the 12% pools in the forum's shared account — "返还" would read as
             a personal rebate. -->
        <p class="text-default-500 mt-0.5 text-xs">
          本站不从中获得任何收益, 全部分成用于回馈用户
        </p>
      </div>

      <KunDivider />

      <div class="mt-2 space-y-1">
        <!-- Step 1 — claim first. -->
        <a
          :href="couponUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="hover:bg-default-100 flex items-start gap-3 rounded-lg p-2 transition-colors"
        >
          <span
            class="bg-primary/10 text-primary flex size-8 shrink-0 items-center justify-center rounded-lg"
          >
            <KunIcon name="lucide:ticket" />
          </span>
          <span class="min-w-0">
            <span class="text-default-800 flex items-center gap-1.5 text-sm">
              领取优惠券
              <KunChip color="primary" variant="flat" size="sm"
                >建议先领</KunChip
              >
            </span>
            <span class="text-default-500 mt-0.5 block text-xs">
              每 3 个月一张 ¥400 券, 单笔满 ¥1200 可用
            </span>
          </span>
        </a>

        <!-- Step 2 — then buy this game. -->
        <a
          :href="purchaseUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="hover:bg-default-100 flex items-start gap-3 rounded-lg p-2 transition-colors"
        >
          <span
            class="bg-primary/10 text-primary flex size-8 shrink-0 items-center justify-center rounded-lg"
          >
            <KunIcon name="lucide:shopping-cart" />
          </span>
          <span class="min-w-0">
            <span class="text-default-800 block text-sm">前往 DLsite 购买</span>
            <!-- The 12% is NOT a personal rebate: DLsite cannot convert the
                 revenue share into per-user discounts, so it becomes points in
                 the forum's shared account. But "不入个人账户" alone reads as if
                 the forum pockets it — the points buy games that get shared with
                 everyone, so the line has to end on where the value lands, not on
                 what the buyer does not get. -->
            <span class="text-default-500 mt-0.5 block text-xs">
              打开本作商品页。订单额 12% 返还为点数, 汇入论坛公共账户,
              用于购买游戏分享给大家
            </span>
          </span>
        </a>
      </div>

      <template v-if="KUN_DLSITE_ANNOUNCE_TOPIC_ID">
        <KunDivider class-name="my-2" />
        <div class="px-2 pb-1">
          <KunLink size="sm" :to="`/topic/${KUN_DLSITE_ANNOUNCE_TOPIC_ID}`">
            了解这次合作的全部细节
          </KunLink>
        </div>
      </template>
    </div>
  </KunPopover>

  <!-- No coupon configured → nothing to choose between; link straight out. -->
  <KunTooltip
    v-else
    text="通过本站链接购买, 全部分成用于回馈用户, 本站不取分成"
  >
    <KunButton
      :href="purchaseUrl"
      target="_blank"
      rel="noopener noreferrer"
      variant="flat"
      color="primary"
      size="sm"
    >
      <span class="flex items-center gap-1">
        <KunIcon name="lucide:shopping-cart" />正版购买
      </span>
    </KunButton>
  </KunTooltip>
</template>
