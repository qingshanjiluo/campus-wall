import { z } from 'zod'
import { KUN_UPDATE_LOG } from '~/constants/update'

export const getUpdateLogSchema = z.object({
  page: z.coerce.number<number>().min(1).max(9999999),
  limit: z.coerce.number<number>().min(1).max(30)
})

// All four locale fields are accepted by the BE CreateHistoryRequest DTO
// (content_en_us / content_ja_jp / content_zh_cn / content_zh_tw).
// Omitting ja-jp / zh-tw silently denied Japanese and Traditional Chinese
// readers any update notes — added them here and in HistoryModal.vue.
export const createUpdateLogSchema = z.object({
  version: z.string().min(1).max(20, '更新版本号最多 20 个字符'),
  content_en_us: z
    .string()
    .max(1000, '更新英语描述最多 1000 个字符')
    .optional()
    .default(''),
  content_ja_jp: z
    .string()
    .max(1000, '更新日语描述最多 1000 个字符')
    .optional()
    .default(''),
  content_zh_cn: z
    .string()
    .max(1000, '更新简体中文描述最多 1000 个字符')
    .optional()
    .default(''),
  content_zh_tw: z
    .string()
    .max(1000, '更新繁体中文描述最多 1000 个字符')
    .optional()
    .default(''),
  type: z.enum(KUN_UPDATE_LOG)
})

export const updateUpdateLogSchema = createUpdateLogSchema.extend({
  update_log_id: z.coerce.number<number>().min(1).max(9999999)
})
