import type { z } from 'zod'
import type { updateToolsetSchema } from '~/validations/toolset'

export type UpdateFormType = z.infer<typeof updateToolsetSchema>

export const toolsetUpdateForm = reactive<UpdateFormType>({
  toolset_id: 0,
  name: '',
  description: '',
  language: 'zh-cn',
  platform: 'windows',
  type: 'emulator',
  version: 'stable',
  homepage: [] as string[],
  aliases: [] as string[]
})
