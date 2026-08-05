<script setup lang="ts">
import type { KunTagInputInvalidReason } from '@kungal/ui-vue'
const props = defineProps<{
  type: 'create' | 'rewrite'
}>()

const { galgamePR } = storeToRefs(useTempGalgamePRStore())
const { aliases } = storeToRefs(usePersistEditGalgameStore())

const selectedAlias = computed({
  get: () =>
    props.type === 'create'
      ? aliases.value
      : (galgamePR.value[0]?.alias ?? []),
  set: (next: string[]) => {
    if (props.type === 'create') {
      aliases.value = next
    }
    if (galgamePR.value[0]) {
      galgamePR.value[0].alias = next
    }
  }
})

const onInvalid = (reason: KunTagInputInvalidReason) => {
  if (reason === 'duplicate') useMessage(10505, 'warn')
  else if (reason === 'max-reached') useMessage(10508, 'warn')
  else if (reason === 'too-long') useMessage(10509, 'warn')
}
</script>

<template>
  <div class="space-y-2">
    <h2 class="text-xl">游戏别名</h2>
    <KunTagInput
      v-model="selectedAlias"
      :max-tags="17"
      :max-tag-length="500"
      :transform="(raw) => raw.toLowerCase()"
      placeholder="请输入游戏别名"
      description="别名最多 17 个, 可以输入别名按下回车创建别名"
      color="primary"
      @invalid="onInvalid"
    />
  </div>
</template>
