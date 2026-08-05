import { z } from 'zod'

export const deleteMessageSchema = z.object({
  message_id: z.coerce.number<number>().min(1).max(9999999)
})

export const getChatMessageHistorySchema = z.object({
  receiver_id: z.coerce.number<number>().min(1).max(9999999),
  page: z.coerce.number<number>().min(1).max(9999999),
  limit: z.coerce.number<number>().min(1).max(30)
})
