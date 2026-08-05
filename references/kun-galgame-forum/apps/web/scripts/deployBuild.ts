import { execSync } from 'child_process'
import { config } from 'dotenv'
import { envSchema } from './dotenv-check'
import { fileURLToPath } from 'url'
import { dirname } from 'path'
import * as fs from 'fs'
import * as path from 'path'

const __filename = fileURLToPath(import.meta.url)
const __dirname = dirname(__filename)

const envPath = path.resolve(__dirname, '..', '.env')
if (!fs.existsSync(envPath)) {
  console.error('.env file not found in the project root.')
  process.exit(1)
}

config({ path: envPath })

try {
  envSchema.safeParse(process.env)

  console.log('Environment variables are valid.')
  console.log('Executing the commands...')

  execSync(
    // `pnpm prisma:push` removed — DB schema is owned by the Go API now
    // (apps/api/migrations). Run `pnpm migrate` from the repo root only
    // when the schema actually changes; most deploys don't touch it.
    'git pull && pnpm build:limit && pnpm stop && pnpm start',
    {
      stdio: 'inherit'
    }
  )
} catch (error) {
  console.error('Invalid environment variables', error)
  process.exit(1)
}
