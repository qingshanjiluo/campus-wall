/**
 * Unified API response format from Go backend.
 * All endpoints return: { code: 0, message: "成功", data: T }
 */
interface KunApiResponse<T> {
  code: number
  message: string
  data: T
}

// Hard ceiling for SSR API calls. Without it, a stalled server-side $fetch to
// the Go backend (a half-open keep-alive socket left over after an api
// redeploy, a transient network stall) never settles: the awaiting
// useAsyncData/Suspense render hangs forever, pinning its entire SSR render
// context in the heap and leaking the in/out sockets (incoming CLOSE_WAIT +
// outgoing ESTABLISHED to the api). Node never reaps a slow handler, so these
// accumulate over days until the event loop is GC-thrashing at 100% CPU.
// ofetch's `timeout` aborts via AbortController, so a stuck request fails fast
// and the handler unwinds. SSR-only on purpose: a hung client fetch can't leak
// the server, and client paths (large file uploads) legitimately run long.
const SSR_API_TIMEOUT_MS = 10000

const CODE_AUTH_EXPIRED = 205
// CODE_BANNED comes from kungal's mapping of OAuth 10014. We MUST NOT
// redirect banned users to /login — a fresh login would just hit 10014
// again at the next refresh, putting them in an infinite loop. Instead
// surface the message prominently and stop there.
const CODE_BANNED = 234

// Business codes the Go backend returns when an action needs a logged-in user
// (distinct from 205 = a dead/expired session). The client gates most of these
// BEFORE the request via requireLogin(), but this is the safety net: any that
// slip through (an ungated control, a direct API call) pop the login modal for
// a logged-out user instead of a dead-end toast. Mirrors the "您需要登录…" set
// in app/error/kunMessage.ts.
const LOGIN_REQUIRED_CODES = new Set([
  10115, 10146, 10216, 10220, 10228, 10232, 10235, 10237, 10240, 10249, 10529,
  10532, 10546
])

// Forward a SMALL ALLOWLIST of cookies to the Go backend during SSR — never
// the whole jar.
//
// pinia-plugin-persistedstate serializes every persisted Pinia store into its
// own browser cookie (user profile, editor drafts, sidebar state, etc.).
// Accumulated, the full Cookie header easily exceeds Fiber's ReadBufferSize —
// manifests as `Request Header Fields Too Large` on Go and silently-empty SSR
// renders (the data fetch fails, the page hydrates with no payload). So we
// forward ONLY the cookies the backend actually reads:
//
//   kungal_session     — auth session: per-user flags (isLiked / isFavorited /
//                        isUpvoted) and any authed read.
//   KUNGalgameSettings — content-rating preference. utils.IsSFW(c) reads
//                        `showKUNGalgameContentLimit` from it and DEFAULTS TO
//                        SFW when the cookie is absent. Without forwarding it,
//                        an SSR refresh renders the SFW-filtered view: NSFW
//                        galgame names in the home "最新动态" feed have no wiki
//                        brief and fall back to "galgame#<id>", while client-
//                        side navigation (full cookie jar) shows the real name.
const SSR_FORWARDED_COOKIES = ['kungal_session', 'KUNGalgameSettings']

const extractForwardedCookies = (
  cookieHeader?: string
): string | undefined => {
  if (!cookieHeader) return undefined
  const kept: string[] = []
  for (const part of cookieHeader.split(';')) {
    const trimmed = part.trim()
    if (SSR_FORWARDED_COOKIES.some((name) => trimmed.startsWith(`${name}=`))) {
      kept.push(trimmed)
    }
  }
  return kept.length > 0 ? kept.join('; ') : undefined
}

// Client-only debounced auth-expiry handling. The Go auth middleware
// deliberately returns transient 205s for the brief window while it refreshes
// the OAuth access token — the session stays intact and recovers (see
// apps/api/internal/middleware/auth.go: "we'd rather get many 205s during a
// refresh window"). A page refresh fires many authed requests into that window
// at once, so reacting to each 205 spammed "登录失效" toasts and reset a user
// who was never actually logged out. Instead the FIRST 205 arms a single timer
// that, after the window has passed, RE-VERIFIES the session with one fresh
// authed request and only logs out if it's genuinely dead. Concurrent 205s in
// the same window collapse into that one pending check (authExpiryTimer latch).
let authExpiryTimer: ReturnType<typeof setTimeout> | null = null

const handleApiError = async (code: number, message: string) => {
  if (import.meta.server) return

  if (code === CODE_BANNED) {
    const userStore = usePersistUserStore()
    if (userStore.id) {
      userStore.resetUser()
    }
    useMessage(message || '您的账号已被封禁', 'error', 10000)
    return
  }

  if (code === CODE_AUTH_EXPIRED) {
    const userStore = usePersistUserStore()
    // Nothing to log out, or a re-check is already pending for this expiry
    // window (the latch that collapses a refresh's burst of 205s into one).
    if (!userStore.id || authExpiryTimer) {
      return
    }
    // Capture the Nuxt context + config now (still inside the request's call
    // stack) so the deferred re-verify / navigate work from the bare timer.
    const nuxtApp = useNuxtApp()
    const config = useRuntimeConfig()
    authExpiryTimer = setTimeout(async () => {
      authExpiryTimer = null
      const store = usePersistUserStore()
      if (!store.id) {
        return
      }
      // The refresh window has passed; re-verify once with a fresh authed
      // request. Raw $fetch (not kunFetch) so a 205 here can't recurse back
      // into this handler. Code 0 → session was only mid-refresh, stay logged
      // in silently. A 401/403 → genuinely dead, log out once. Any other
      // failure (network blip) is left alone — we don't log out on a maybe.
      let dead = false
      try {
        const resp = await $fetch<KunApiResponse<unknown>>(
          `${config.public.apiBaseUrl}/api/user/status`,
          { credentials: 'include' }
        )
        dead = !resp || resp.code !== 0
      } catch (e) {
        const status =
          (e as { status?: number; response?: { status?: number } })?.status ??
          (e as { response?: { status?: number } })?.response?.status
        dead = status === 401 || status === 403
      }
      if (!dead || !store.id) {
        return
      }
      store.resetUser()
      useMessage(message || '登录已失效，请重新登录', 'error', 7777)
      nuxtApp.runWithContext(() => navigateTo('/'))
    }, 1500)
    return
  }

  // Login-required business error for a logged-out user → pop the login modal
  // (the same one requireLogin() opens) instead of a toast. A logged-IN user
  // hitting one of these is a real error, so fall through to the message.
  if (LOGIN_REQUIRED_CODES.has(code) && !usePersistUserStore().id) {
    useAuthModal().open()
    return
  }

  if (code !== 0) {
    useMessage(message, 'error')
  }
}

/**
 * useKunFetch — SSR-safe composable built on Nuxt 4 `createUseFetch`.
 *
 * Automatically:
 * - Resolves baseURL for SSR/CSR
 * - Forwards cookies during SSR via credentials
 * - Unwraps `{ code, data }` response
 * - Handles auth/biz errors client-side only
 *
 * The response type T is what the Go backend returns inside `data`.
 * The transform unwraps it so `data.value` is `T | null` directly.
 *
 * @example
 * const { data } = await useKunFetch<HomeData>('/home')
 * // data.value?.galgames
 *
 * @example
 * const { data } = await useKunFetch<{ items: T[], total: number }>(
 *   '/topic',
 *   { query: pageData }
 * )
 */
export const useKunFetch = createUseFetch({
  // SSR-only ceiling so a stalled fetch can't hang the render and leak its
  // context. import.meta.server is build-time constant → client bundle gets
  // `undefined` (no timeout). See SSR_API_TIMEOUT_MS.
  timeout: import.meta.server ? SSR_API_TIMEOUT_MS : undefined,
  credentials: 'include',
  // SSR is cross-origin from Nuxt → Go API, so `credentials: 'include'` is
  // a no-op on the server. Manually forward the allowlisted cookies (session +
  // content-rating preference) so per-user flags (isLiked / isFavorited /
  // isUpvoted) AND the SFW filter render correctly on first paint — see
  // SSR_FORWARDED_COOKIES above for why we don't bundle the whole jar.
  onRequest({ options }) {
    // baseURL is set HERE, not as a `baseURL` factory option, on purpose. As an
    // option it gets hashed into useFetch's cache key — and it differs between
    // SSR (internal http://kungal-api:2334) and the client (public
    // https://www.kungal.com). That divergence made the client's key miss the
    // SSR payload, so EVERY page re-fetched its data on hydration: the page
    // re-rendered after the await ("loads twice") and Vue logged a hydration
    // mismatch. Applying baseURL in onRequest keeps the cache key path-only and
    // identical on both sides, so the SSR payload is reused and nothing re-fetches.
    const config = useRuntimeConfig()
    options.baseURL = `${
      import.meta.server ? config.apiBaseUrl : config.public.apiBaseUrl
    }/api`
    if (import.meta.server) {
      const forwarded = extractForwardedCookies(
        useRequestHeaders(['cookie']).cookie
      )
      if (forwarded) {
        const merged = new Headers(options.headers as HeadersInit | undefined)
        merged.set('cookie', forwarded)
        options.headers = merged
      }
    }
  },
  async onResponseError({ response }) {
    const resp = response._data as KunApiResponse<unknown> | undefined
    if (resp && resp.code !== 0) {
      await handleApiError(resp.code, resp.message)
    }
  },
  transform(resp: unknown) {
    const envelope = resp as KunApiResponse<unknown> | null | undefined
    if (!envelope || envelope.code !== 0) {
      return null
    }
    // Go's response.OKMessage omits `data`. Callers typically gate optimistic
    // updates on `if (result)`, so returning the message keeps the truthy
    // success signal intact while still distinguishing from the `null`
    // returned for real failures.
    return envelope.data !== undefined ? envelope.data : envelope.message
  }
})

/**
 * kunFetch — Imperative fetch for mutations (button clicks, form submits).
 * Client-side only. Unwraps { code, data } and handles errors.
 * Returns the unwrapped data, or null on error.
 *
 * @example
 * const result = await kunFetch<string>('/user/bio', {
 *   method: 'PUT',
 *   body: { bio: 'hello' }
 * })
 * if (result) { useMessage('更新成功', 'success') }
 */
export const kunFetch = async <T>(
  url: string,
  options?: Record<string, unknown>
): Promise<T | null> => {
  const config = useRuntimeConfig()
  // Prefer the server-only base URL when running on SSR — it bypasses
  // the public proxy and goes straight to the Go backend.
  const apiBase = import.meta.server
    ? `${config.apiBaseUrl}/api`
    : `${config.public.apiBaseUrl}/api`

  // Cookie forwarding (SSR): forward only the allowlisted cookies (session +
  // content-rating preference). Bundling the whole cookie header (Pinia
  // persisted stores, color-mode, etc.) can blow past Fiber's header limit —
  // same rationale as useKunFetch / SSR_FORWARDED_COOKIES above.
  const headers = new Headers(
    (options as { headers?: HeadersInit } | undefined)?.headers
  )
  if (import.meta.server) {
    const forwarded = extractForwardedCookies(
      useRequestHeaders(['cookie']).cookie
    )
    if (forwarded) {
      headers.set('cookie', forwarded)
    }
  }

  try {
    const resp = await $fetch<KunApiResponse<T>>(`${apiBase}${url}`, {
      // SSR-only default (see SSR_API_TIMEOUT_MS); listed before ...options so a
      // caller can still pass its own timeout (e.g. a long upload).
      timeout: import.meta.server ? SSR_API_TIMEOUT_MS : undefined,
      credentials: 'include',
      ...options,
      headers
    })

    if (!resp || resp.code !== 0) {
      if (resp) {
        await handleApiError(resp.code, resp.message)
      }
      return null
    }

    // Same fallback rationale as useKunFetch above: OKMessage responses
    // have no `data`, but callers check `if (result)` to confirm success.
    return resp.data !== undefined ? resp.data : (resp.message as T)
  } catch (error) {
    if (import.meta.client) {
      // ofetch rejects on ANY non-2xx status, so an app error that the Go
      // backend serves with an error status — e.g. an expired session is
      // 401 + { code: 205 } (errors.ErrAuthExpired) — lands here, NOT in the
      // `resp.code !== 0` branch above. Without unwrapping it we'd only ever
      // show "网络请求失败" and never reset the user / redirect to login.
      const err = error as {
        data?: KunApiResponse<unknown>
        status?: number
        statusCode?: number
        response?: { status?: number; _data?: KunApiResponse<unknown> }
      }
      // ofetch exposes the parsed body as both `err.data` and
      // `err.response._data`; read either so a wrapped instance can't hide it.
      const envelope = err.data ?? err.response?._data
      const status = err.status ?? err.statusCode ?? err.response?.status

      if (envelope && envelope.code !== 0) {
        // App envelope on a non-2xx — route through the same handler as a
        // 2xx-wrapped error (205 → logout, 234 → banned, else → message).
        await handleApiError(envelope.code, envelope.message)
      } else if (status === 401 || status === 403) {
        // Auth-failure HTTP status but no parseable envelope (a reverse-proxy
        // error page, a stripped body). Still treat as auth-expiry so a dead
        // session logs the user out instead of looking like a flaky network.
        await handleApiError(CODE_AUTH_EXPIRED, '登录已失效，请重新登录')
      } else {
        // Genuine transport failure: no HTTP response reached the browser
        // (offline, DNS, connection reset, CORS block). Don't force a logout
        // on a network blip — just surface the generic message.
        useMessage('网络请求失败，请稍后重试', 'error')
      }
    }
    return null
  }
}
