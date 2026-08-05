import type { Ref } from 'vue'

// Local "known accounts on this device" cache that powers the account-switch
// submenu (see docs/oauth/09-account-switching.md §3.6). Forum is a BFF — the
// browser holds NO token — so the menu list comes from this localStorage cache
// and switching always goes through a top-level authorize redirect. The OP's
// session bag is the source of truth for what actually happens on click. The
// cache holds NO tokens: just enough to render an avatar + name + a `login_hint`.
// It's "best-effort accurate": an account added elsewhere only appears here after
// you've switched to it once on forum, and an account logged out elsewhere
// gracefully falls back to login on click.
export interface KnownAccount {
  // OAuth subject UUID — the switch `login_hint` + the cache identity key.
  sub: string
  // Local DB id — used only to mark which entry is the currently-active account.
  id: number
  name: string
  // Pre-resolved avatar URL (KunAvatar reads `user.avatar` directly).
  avatar: string
  roles: string[]
}

const STORAGE_KEY = 'kun-galgame-known-accounts'
// A browser realistically juggles a handful of accounts; cap so a long-lived
// device can't grow the list unbounded. Most-recently-used stay at the front.
const MAX_ACCOUNTS = 8

// Module-global so the localStorage read happens exactly once per page load.
let loaded = false

const read = (): KnownAccount[] => {
  if (!import.meta.client) return []
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    const parsed = raw ? JSON.parse(raw) : []
    return Array.isArray(parsed) ? parsed.filter((a) => a && a.sub) : []
  } catch {
    // Corrupt / unavailable storage (private mode, quota) → empty list.
    return []
  }
}

// Merge stored entries into the live list WITHOUT clobbering anything added
// earlier this tick (a fresh login can rememberUser before the deferred load
// runs — those entries must win and survive). Idempotent via the `loaded` flag.
const ensureLoaded = (accounts: Ref<KnownAccount[]>) => {
  if (loaded || !import.meta.client) return
  loaded = true
  const seen = new Set(accounts.value.map((a) => a.sub))
  accounts.value = [
    ...accounts.value,
    ...read().filter((a) => !seen.has(a.sub))
  ].slice(0, MAX_ACCOUNTS)
}

export const useKnownAccounts = () => {
  // useState → SSR-safe shared singleton. Starts [] on the server AND on the
  // first client render (no localStorage), so there's no hydration mismatch; the
  // deferred onMounted load fills it in right after mount.
  const accounts = useState<KnownAccount[]>('kun-known-accounts', () => [])
  onMounted(() => ensureLoaded(accounts))

  const persist = () => {
    if (!import.meta.client) return
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(accounts.value))
    } catch {
      // Storage unavailable — the in-memory list still works for this session.
    }
  }

  // Insert/refresh an account, moving it to the front (most-recent-first).
  const upsert = (account: KnownAccount) => {
    if (!account.sub || !account.id) return
    // Pull stored history in first, or this write would persist a list missing
    // every previously-known account.
    ensureLoaded(accounts)
    accounts.value = [
      account,
      ...accounts.value.filter((a) => a.sub !== account.sub)
    ].slice(0, MAX_ACCOUNTS)
    persist()
  }

  // Forget every account on THIS device. Called on full ("everywhere") logout so
  // a shared / public terminal doesn't leave the account roster in localStorage.
  // Local-only session clears keep it — the OP bag is intact, so the list stays
  // useful for a quick re-login.
  const clearAll = () => {
    accounts.value = []
    loaded = true // the empty in-memory list is now authoritative
    if (!import.meta.client) return
    try {
      localStorage.removeItem(STORAGE_KEY)
    } catch {
      // Storage unavailable — the in-memory clear still holds for this session.
    }
  }

  // Snapshot the freshly-authenticated user into the cache. Called after every
  // successful login / switch (both return through /auth/callback), so the
  // active account stays current. A blank name (degraded /auth/me) is skipped so
  // a transient blip can't overwrite a good cached entry with a nameless row.
  const rememberUser = (user: {
    sub?: string
    id?: number
    name?: string
    avatar?: string
    roles?: string[]
  }) => {
    if (!user.sub || !user.id || !user.name) return
    upsert({
      sub: user.sub,
      id: user.id,
      name: user.name,
      avatar: user.avatar || '',
      roles: user.roles ?? []
    })
  }

  return { accounts, upsert, clearAll, rememberUser }
}
