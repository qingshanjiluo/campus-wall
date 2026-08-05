<script setup lang="ts">
import { submitGalgameSchema } from '~/validations/galgame'

// POST /galgame/submit mints a registry work in the `pending` claim state and
// records the birth as a claim event, so "who submitted this and when" is a
// fact rather than a column somebody has to remember to fill. The legacy admin
// direct-publish endpoint is gone: publishing and submitting are one lifecycle
// now, so a moderator submits here too and approves from the queue.
//
// The post-success redirect goes to /edit/galgame/mine — a pending submission
// has no public page to land on.

// No vndb_id: submission is exclusively for VNDB-unlisted works (wiki has
// the full VNDB set as claimable drafts already) — see Galgame.vue.
const {
  name,
  content_limit,
  age_limit,
  original_language,
  introduction,
  aliases,
  release_date,
  release_date_tba
} = storeToRefs(usePersistEditGalgameStore())

const isPublishing = ref(false)

const handleSubmitGalgame = async () => {
  const banner = await getImage('kun-galgame-publish-banner')
  // Wire-format payload uses snake_case keys to match the wiki API
  // (POST /galgame/submit). The Vue store keeps camelCase locally; we
  // rename at the boundary so the schema, the JSON body, and the wiki
  // contract all agree.
  const data: Record<
    string,
    number | string | string[] | Blob | boolean | null
  > = {
    name_en_us: name.value['en-us'],
    name_ja_jp: name.value['ja-jp'],
    name_zh_cn: name.value['zh-cn'],
    name_zh_tw: name.value['zh-tw'],
    intro_en_us: introduction.value['en-us'],
    intro_ja_jp: introduction.value['ja-jp'],
    intro_zh_cn: introduction.value['zh-cn'],
    intro_zh_tw: introduction.value['zh-tw'],
    content_limit: content_limit.value,
    age_limit: age_limit.value,
    original_language: original_language.value,
    // U1: empty string = unknown; wiki schema accepts "" or YYYY-MM-DD.
    release_date: release_date.value,
    release_date_tba: release_date_tba.value,
    banner
  }
  const result = submitGalgameSchema.safeParse(data)
  if (!result.success) {
    const message = JSON.parse(result.error.message)[0]
    useMessage(
      formatKunZodIssue(message),
      'warn'
    )
    return
  }
  const res = await useComponentMessageStore().alert(
    '确定提交 Galgame 申请吗?',
    '提交后将进入审核队列, 审核通过后才会被公开展示。审核结果会通过站内消息通知您。在审核期间您可以在「我的提交」页继续编辑或撤回。'
  )
  if (!res) {
    return
  }

  if (isPublishing.value) {
    return
  } else {
    isPublishing.value = true
    useMessage(10525, 'info', 7777)
  }

  // The banner is uploaded FIRST and travels as a hash. A cover is a reference
  // to bytes that must already exist, so it cannot ride the mint — it is
  // attached as the submission's first edit, which is also how the reviewer
  // sees it alongside the rest.
  const { banner: _bannerBlob, ...jsonFields } = data
  let bannerHash = ''
  if (banner instanceof File) {
    const uploaded = await uploadGalgameImage(banner, 'galgame_banner', banner.name)
    if (uploaded) {
      bannerHash = uploaded.hash
    }
  }
  const created = await kunFetch<{ gid: number; claim_state: string }>(
    '/galgame/submit',
    {
      method: 'POST',
      body: {
        ...jsonFields,
        aliases: aliases.value,
        banner_hash: bannerHash
      }
    }
  )
  isPublishing.value = false

  if (created?.gid) {
    await deleteImage('kun-galgame-publish-banner')
    // Clear the persisted wizard-step draft key (set by Galgame.vue via
    // useLocalStorage) so the next "new submission" starts at step ①
    // rather than resuming a stale position over empty fields.
    if (import.meta.client) {
      localStorage.removeItem('kun-galgame-publish-step')
    }

    useKunLoliInfo('Galgame 申请已提交, 等待审核', 5)
    await navigateTo('/edit/galgame/mine')
    usePersistEditGalgameStore().resetEditGalgameStore()
  }
}
</script>

<template>
  <div class="flex justify-end">
    <KunButton
      :disabled="isPublishing"
      :loading="isPublishing"
      size="lg"
      @click="handleSubmitGalgame"
    >
      提交 Galgame 申请
    </KunButton>
  </div>
</template>
