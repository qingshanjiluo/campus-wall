import { z } from 'zod'

// Localize every DEFAULT Zod message (min / max / type / email / url …) to
// 简体中文, app-wide, in one place. Custom `.message` overrides still win — this
// is only the fallback so an uncustomised validator (e.g. `overall.min(1)`)
// never surfaces an English string to a Chinese user. Field labels are added
// separately by formatKunZodIssue (app/utils/kunZodError.ts).
export default defineNuxtPlugin(() => {
  z.config(z.locales.zhCN())
})
