// Shared "kick off an OAuth flow" entry points. Login, register, AND account
// switching all generate the same PKCE bundle + write the same cookies
// (oauth_code_verifier / oauth_state) that pages/auth/callback.vue reads back
// when the browser returns from OAuth. The only differences are the `prompt` /
// `login_hint` params and WHERE the browser lands first:
//
//   - login         → ${oauthServerUrl}/oauth/authorize?<params>
//   - switch account → …authorize?prompt=select_account&login_hint=<sub>
//   - add account    → …authorize?prompt=login
//   - register       → ${oauthFrontendUrl}/auth/register?redirect=<encoded(authorize)>
//
// Account switching reuses the SAME server-side callback (forum is a BFF — the
// token lives in Redis, never the browser), so a switch is just another authorize
// redirect; the OP picks the account from its session bag and the callback swaps
// the kungal session. See docs/oauth/09-account-switching.md.

import Cookies from 'js-cookie'

interface OAuthFlowOptions {
  // OIDC prompt: 'select_account' (switch — show the OP account picker) or
  // 'login' (add — force a fresh OP login, no silent re-consent).
  prompt?: string
  // The target account's OAuth sub, so the OP can switch straight to it.
  loginHint?: string
  // Where to send the browser after the callback completes (a switch should land
  // back on the page you were on). Consumed + origin-guarded by consumeOAuthReturnTo.
  returnTo?: string
}

const buildAuthorizeUrl = async (
  opts: OAuthFlowOptions = {}
): Promise<string> => {
  const config = useRuntimeConfig()

  const codeVerifier = generateCodeVerifier()
  const codeChallenge = await generateCodeChallenge(codeVerifier)
  const state = generateState()

  // Persist the per-attempt secrets so /auth/callback can validate the
  // returned `state` and complete the PKCE exchange.
  //
  // Stored in COOKIES, not sessionStorage: sessionStorage is per-tab and is NOT
  // reliably preserved across the cross-origin OAuth redirect on lightweight /
  // older mobile browsers (Via, in-app WebViews, COOP / process switches, or
  // when the auth page opens in a new tab). The callback then reads null and
  // fails with "State 不匹配". Cookies are origin-scoped, survive the round-trip,
  // and auto-expire. 10 min = the auth round-trip window; `secure` only on https
  // so dev over http still sets them. Every entry point writes the same keys so
  // the callback never has to know which one started the flow.
  const oauthCookieOptions = {
    expires: 10 / 1440,
    sameSite: 'lax' as const,
    secure: location.protocol === 'https:',
    path: '/'
  }
  Cookies.set('oauth_code_verifier', codeVerifier, oauthCookieOptions)
  Cookies.set('oauth_state', state, oauthCookieOptions)
  // Stash the post-callback return path for switch/add so the user lands back
  // where they were (consumed + origin-guarded by consumeOAuthReturnTo).
  if (opts.returnTo) {
    Cookies.set('oauth_return_to', opts.returnTo, oauthCookieOptions)
  }

  const params = new URLSearchParams({
    client_id: config.public.oauthClientId as string,
    redirect_uri: config.public.oauthRedirectUri as string,
    response_type: 'code',
    scope: 'openid profile',
    state,
    code_challenge: codeChallenge,
    code_challenge_method: 'S256'
  })
  if (opts.prompt) params.set('prompt', opts.prompt)
  if (opts.loginHint) params.set('login_hint', opts.loginHint)
  return `${config.public.oauthServerUrl}/oauth/authorize?${params}`
}

export const startOAuthLogin = async (
  opts: OAuthFlowOptions = {}
): Promise<void> => {
  const authorizeUrl = await buildAuthorizeUrl(opts)
  window.location.href = authorizeUrl
}

// Switch to an account the OP already holds for this browser (login_hint = its
// sub). The OP switches silently while the account is still in its session bag,
// or forces a re-login for an admin (step-up) / an account logged out elsewhere.
// Either way the browser returns through /auth/callback as whichever account the
// OP yields — the top-bar avatar (from the callback) is the source of truth.
// See docs/oauth/09-account-switching.md §3.1 / §3.6.
export const startOAuthSwitchAccount = async (
  loginHint: string,
  returnTo?: string
): Promise<void> => {
  await startOAuthLogin({ prompt: 'select_account', loginHint, returnTo })
}

// Add a brand-new account: force the OP login screen (don't silently re-consent
// the current session). The new account joins the OP bag + our local list.
export const startOAuthAddAccount = async (
  returnTo?: string
): Promise<void> => {
  await startOAuthLogin({ prompt: 'login', returnTo })
}

export const startOAuthRegister = async (): Promise<void> => {
  const config = useRuntimeConfig()
  const authorizeUrl = await buildAuthorizeUrl()
  const registerUrl = `${config.public.oauthFrontendUrl}/auth/register?redirect=${encodeURIComponent(authorizeUrl)}`
  window.location.href = registerUrl
}

// RP-initiated logout. Resetting the forum's own user store is NOT enough: the
// central OP (oauth.kungal.com) session survives (its localStorage user is
// cross-origin and its refresh cookie is cross-site), so the next login would
// silently re-consent (first-party auto_consent) straight back into the same
// account. After clearing local state, callers top-level navigate here, which
// sends the browser to the OP logout entrypoint (`{oauthServerUrl}/oauth/logout`,
// symmetric with /oauth/authorize). The OP clears its session and redirects back
// to `redirect` (validated against this client's registered redirect_uris).
// See docs/oauth/07-logout.md.
export const startOAuthLogout = (): void => {
  const config = useRuntimeConfig()
  const params = new URLSearchParams({
    client_id: config.public.oauthClientId as string,
    redirect: `${window.location.origin}/`
  })
  window.location.href = `${config.public.oauthServerUrl}/oauth/logout?${params}`
}

// Read (and clear) the post-callback return path stashed by a switch/add flow.
// Returns null when absent or unsafe — the caller falls back to a safe default.
//
// Open-redirect guard: resolve the value against our own origin and require the
// result to stay same-origin, then return ONLY the path portion so the host can
// never be attacker-controlled (closes protocol-relative "//x", backslash,
// scheme "javascript:", and percent-encoded bypasses). The value is ours (set
// from route.fullPath) but rides in a JS-writable cookie, so it's treated as
// untrusted per docs/oauth/09-account-switching.md §4. Client-side only
// (callback.vue is ssr:false), so window.location.origin is always available.
export const consumeOAuthReturnTo = (): string | null => {
  const value = Cookies.get('oauth_return_to')
  if (value) Cookies.remove('oauth_return_to', { path: '/' })
  if (!value) return null
  try {
    const url = new URL(value, window.location.origin)
    if (url.origin === window.location.origin) {
      return url.pathname + url.search + url.hash
    }
  } catch {
    // Malformed → fall through to the caller's default.
  }
  return null
}
