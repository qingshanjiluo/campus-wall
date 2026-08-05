import type { ZodSchema } from 'zod'

export const useKunSchemaValidator = <T>(
  schema: ZodSchema<T>,
  data: unknown
) => {
  const result = schema.safeParse(data)
  if (!result.success) {
    const message = JSON.parse(result.error.message)[0]
    useMessage(
      formatKunZodIssue(message),
      'warn'
    )
    return
  }
  return result.data
}
