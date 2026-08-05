import { z } from 'zod'
import { KUN_ALLOWED_ACTIVITY_TYPE } from '~/constants/activity'

export const getActivitySchema = z.object({
  // Opaque keyset cursor from the previous page's next_cursor; empty = first page.
  cursor: z.string().optional(),
  limit: z.coerce.number<number>().min(1).max(50),
  type: z.enum(KUN_ALLOWED_ACTIVITY_TYPE)
})

export const getActivityTimelineSchema = z.object({
  cursor: z.string().optional(),
  limit: z.coerce.number<number>().min(1).max(50)
})
