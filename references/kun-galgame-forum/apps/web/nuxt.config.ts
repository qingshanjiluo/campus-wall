import tailwindcss from '@tailwindcss/vite'
import { ICON_COLLECTIONS, ICON_NAMES } from './lib/icon'
import type { TSConfig } from 'pkg-types'

const sharedTsConfig: TSConfig = {
  exclude: ['**/backup/**', '**/dist/**', '**/node_modules/**']
}

// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  // KunUI is consumed as the published Nuxt layer (@kungal/ui-nuxt): it
  // auto-imports all components & composables from @kungal/ui-vue and wires
  // NuxtLink / @nuxt/icon / @nuxt/image. The app owns its Tailwind entry
  // (app/styles/tailwindcss.css → @kungal/ui-tokens + ui-vue style + @source).
  // @kungal/editor-nuxt auto-imports <KunEditor> (the extracted Milkdown editor)
  // + its KunUI chrome; it assumes ui-nuxt is already extended, so it comes 2nd.
  extends: ['@kungal/ui-nuxt', '@kungal/editor-nuxt'],

  devtools: { enabled: false },

  app: {
    pageTransition: { name: 'kun-page', mode: 'out-in' },
    layoutTransition: { name: 'kun-page', mode: 'out-in' },

    // https://github.com/nuxt/nuxt/issues/26565#issuecomment-3448517709
    baseURL: '/'
    // buildAssetsDir: `/_nuxt/v${Math.floor(Date.now() / 1000).toString()}/`
  },

  experimental: {
    scanPageMeta: true,
    typescriptPlugin: true
  },

  compatibilityDate: '2025-07-15',

  devServer: {
    host: '127.0.0.1',
    port: 2333
  },

  modules: [
    '@nuxt/image',
    '@nuxt/icon',
    '@nuxt/eslint',
    '@nuxtjs/color-mode',
    '@pinia/nuxt',
    '@dxup/nuxt',
    'pinia-plugin-persistedstate/nuxt',
    'nuxt-schema-org',
    '@nuxtjs/sitemap',
    'nuxt-umami'
  ],

  runtimeConfig: {
    // Server-only: used by useFetch during SSR to directly reach Go API
    apiBaseUrl: process.env.API_BASE_URL || 'http://127.0.0.1:2334',

    // Server-only: CDN base for resolving the /image/<hash> content token in the
    // server/routes/image/[hash].ts 302 fallback. Literal default (generic build
    // has no build-arg); override at runtime with NUXT_IMAGE_CDN_BASE. MUST match
    // the Go API's KUN_IMAGE_PUBLIC_BASE_URL so both resolvers agree.
    imageCdnBase:
      process.env.IMAGE_CDN_BASE || 'https://image.kungal.iloveren.link',

    public: {
      KUN_GALGAME_URL: process.env.KUN_GALGAME_URL,
      KUN_VISUAL_NOVEL_FORUM_YANDEX_VERIFICATION:
        process.env.KUN_VISUAL_NOVEL_FORUM_YANDEX_VERIFICATION,

      // Go API base URL (client-side, relative — goes through browser)
      apiBaseUrl: process.env.API_BASE_URL || 'http://127.0.0.1:2334',

      // OAuth
      // oauthServerUrl   : API base (e.g. .../api/v1) — used for HTTP calls.
      // oauthFrontendUrl : web-app root — used for cross-app navigation
      //                    (改邮箱 / 改密码 jumps to /profile). Kept
      //                    distinct because dev runs them on separate
      //                    ports; in prod they share a domain.
      oauthServerUrl:
        process.env.OAUTH_SERVER_URL || 'http://127.0.0.1:9277/api/v1',
      oauthFrontendUrl:
        process.env.OAUTH_FRONTEND_URL || 'https://oauth.kungal.com',
      oauthClientId: process.env.OAUTH_CLIENT_ID || '',
      oauthRedirectUri:
        process.env.OAUTH_REDIRECT_URI || 'http://127.0.0.1:2333/auth/callback',

      // Client-readable mirror of the server-only imageCdnBase above. Lets the FE
      // resolve content-addressed image tokens (/image/<hash>) to an ABSOLUTE CDN
      // URL (utils/imageSrc.ts), so covers / galgame images skip both @nuxt/image
      // IPX (which 404s on the token) AND the /image 302 hop. Same literal default
      // as the server one; override at runtime with NUXT_PUBLIC_IMAGE_CDN_BASE.
      imageCdnBase:
        process.env.IMAGE_CDN_BASE || 'https://image.kungal.iloveren.link'
    }
  },

  routeRules: {
    // Animated reaction emoji are static and rarely change — let the browser
    // disk-cache them across visits (HTTP cache; no need for IndexedDB).
    '/emoji/**': { headers: { 'cache-control': 'public, max-age=2592000' } }
  },

  imports: {
    dirs: ['./composables', './config', './utils'],
    // The old @kun/ui layer auto-imported `cn` (the classname helper) globally;
    // the published @kungal/ui-nuxt layer only auto-imports its Kun* composables,
    // so re-register `cn` (and the shared types) from @kungal/ui-core here — the
    // forum uses `cn(...)` unimported in ~80 components.
    presets: [
      {
        from: '@kungal/ui-core',
        imports: ['cn', 'randomNum']
      }
    ]
  },

  site: {
    // Drives both nuxt-schema-org and @nuxtjs/sitemap (shared nuxt-site-config).
    // Literal fallback for the same reason as `umami.id` below: kungal-web is built
    // GENERIC (CI passes no build-args), so process.env.KUN_GALGAME_URL is undefined
    // at build → an empty site.url would make sitemap <loc>s non-absolute. Prod runs
    // on this canonical host; dev sets KUN_GALGAME_URL in .env.
    url: process.env.KUN_GALGAME_URL || 'https://www.kungal.com'
  },

  sitemap: {
    // Keep private / auth-gated / editor / non-content static routes OUT of the
    // auto-discovered pages. Routes with params (/topic/[id], /user/[id]/…) are
    // never auto-included, so only the param-free pages below need excluding.
    exclude: [
      '/admin',
      '/admin/**',
      '/auth/**',
      '/edit/**',
      '/message',
      '/message/**',
      '/report',
      '/user',
      '/rss'
    ],
    // ~16k URLs → emit a /sitemap_index.xml of ≤1000-URL chunks (Google caps a
    // single file at 50k/50MB; smaller chunks cache + debug better and give us
    // headroom as content grows). In 7.x, `chunks` is only honoured inside the
    // multi-sitemap `sitemaps` map — not on the default single sitemap.
    defaultSitemapsChunkSize: 1000,
    sitemaps: {
      kungal: {
        // Static, param-free pages auto-discovered from the route table.
        includeAppSources: true,
        // Dynamic content URLs (topics/galgames/…) from this runtime endpoint,
        // which enumerates the Go API with the SFW filter forced on. Runs at
        // request time (cached) — never at build, which has no Go-API access.
        sources: ['/api/__sitemap__/urls'],
        chunks: true
      }
    },
    // Cache the rendered sitemap; the source endpoint is cached independently too.
    cacheMaxAgeSeconds: 60 * 60 * 6,
    defaults: { changefreq: 'daily', priority: 0.7 }
  },

  umami: {
    // Public website-id (it ships in the client tracker tag — not a secret). The
    // `|| '<id>'` fallback is the actual fix: kungal-web is built GENERIC (the CI
    // matrix passes no build-args) and .env is gitignored, so at BUILD time
    // process.env.KUN_VISUAL_NOVEL_FORUM_UMAMI_ID was undefined → nuxt-umami baked an
    // undefined id → every /api/send 400'd. nuxt-umami reads `id` at build (a module
    // option), not at runtime, so the default has to be a literal here.
    id:
      process.env.KUN_VISUAL_NOVEL_FORUM_UMAMI_ID ||
      '2fc714ed-ed3b-459a-b52c-65f7e1621834',
    host: 'https://umami.kungal.org/',
    autoTrack: true,
    // One baked id runs in every deployment — keep local dev traffic out of prod stats.
    ignoreLocalhost: true
  },

  // Frontend
  css: ['~/styles/index.css'],

  icon: {
    mode: 'svg',
    // The default local endpoint is /api/_nuxt_icon, but in prod Traefik routes
    // ALL of /api/* to the Go API — so @nuxt/icon's client fallback for any icon
    // not in clientBundle (the ~20 dynamic `:name=expr` bindings) hit Go and
    // 404'd, breaking those icons and spamming the API error log. Move it off
    // the Go-owned /api namespace so it reaches Nitro, which serves it from
    // serverBundle (which carries every collection, so dynamic icons resolve).
    localApiEndpoint: '/_nuxt_icon',
    serverBundle: {
      collections: ICON_COLLECTIONS
    },
    clientBundle: {
      icons: ICON_NAMES,
      scan: false
    }
  },

  typescript: {
    tsConfig: {
      ...sharedTsConfig
    },
    nodeTsConfig: {
      ...sharedTsConfig
    },
    sharedTsConfig: {
      ...sharedTsConfig
    }
  },

  vite: {
    plugins: [tailwindcss()]
    // optimizeDeps: {
    //   include: ['isomorphic-dompurify', 'date-fns/locale']
    // }
  },

  // $production: {
  //   vite: {
  //     esbuild: {
  //       drop: ['console', 'debugger']
  //     }
  //   }
  // },

  // ogImage: {
  //   fonts: [
  //     {
  //       name: 'Lolita',
  //       weight: 400,
  //       path: '/fonts/Lolita.ttf'
  //     }
  //   ]
  // },

  eslint: {
    config: {
      stylistic: false
    }
  },

  // Pinia store functions auto imports
  pinia: {
    storesDirs: ['./store/**']
  },

  piniaPluginPersistedstate: {
    cookieOptions: {
      maxAge: 60 * 60 * 24 * 7,
      sameSite: 'strict'
    }
  },

  colorMode: {
    preference: 'system',
    fallback: 'light',
    globalName: '__KUNGALGAME_COLOR_MODE__',
    componentName: 'ColorScheme',
    classPrefix: 'kun-',
    classSuffix: '-mode',
    storageKey: 'kungalgame-color-mode'
  },

  nitro: {
    typescript: {
      tsConfig: {
        ...sharedTsConfig
      }
    }
  }
})
