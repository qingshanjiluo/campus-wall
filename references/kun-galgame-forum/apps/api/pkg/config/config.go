package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Server         ServerConfig
	Database       DatabaseConfig
	Redis          RedisConfig
	OAuth          OAuthConfig
	FileStorage    S3Config // file storage (B2) — toolset archive uploads
	Mail           MailConfig
	Search         SearchConfig
	CORS           CORSConfig
	NextMoeAPI     NextMoeAPIConfig
	ImageClient    ImageClientConfig
	ArtifactClient ArtifactClientConfig
	LinkChecker    LinkCheckerConfig
	Trust          TrustConfig
	Catalog        CatalogClientConfig
	Community      CommunityConfig
	Dlsite         DlsiteConfig
}

// CommunityConfig holds what kungal needs to reach the infra community primitive
// (kun-galgame-infra cmd/community :9282) — the unconditional backend for the
// galgame + resource (rating / website / toolset) comment areas since the legacy
// in-forum comment routes were retired (charter step 06a). Auth is HTTP Basic
// reusing the OAuth client_id/secret; the community service reads
// oauth_clients.catalog_site to derive kungal's tenant (never on the wire).
// ClientID/ClientSecret default to the OAuth credentials when unset (filled in
// app.go), so a single OAuth client works. An empty BaseURL (or OAuth creds)
// leaves the client unconfigured: comment reads degrade to empty pages and
// writes to 503, so a dev box without a community service still boots.
type CommunityConfig struct {
	BaseURL      string // community S2S base, e.g. http://127.0.0.1:9282/api/v1/community
	ClientID     string // OAuth client id (Basic auth); defaults to OAuth.ClientID
	ClientSecret string // OAuth client secret; defaults to OAuth.ClientSecret
}

// CatalogClientConfig holds what kungal needs to reach the infra Catalog service
// (kun-galgame-infra cmd/catalog :9281): the base URL for the S2S editing-engine
// face (staff/owner writes + the whole review chain). Auth is HTTP Basic reusing
// the OAuth client_id/secret (wired in app.go). Empty BaseURL (or OAuth creds) =
// the integration is inert (those endpoints degrade to 503) so a dev box without
// a catalog service is harmless.
type CatalogClientConfig struct {
	BaseURL string // catalog service base, e.g. http://127.0.0.1:9281
}

// TrustConfig holds what kungal needs to integrate the infra Trust & Safety
// service (kun-galgame-infra :9283): the base URL for submitting reports S2S,
// and the HMAC secret for verifying inbound enforcement callbacks. The S2S
// Basic-auth credentials reuse the OAuth client_id/secret (wired in app.go) —
// the trust service reads oauth_clients.catalog_site to derive kungal's site.
// Empty BaseURL / CallbackSecret = the integration is inert (reports degrade,
// callbacks are rejected) so a dev box without a trust service is harmless.
type TrustConfig struct {
	BaseURL        string // trust service base, e.g. http://127.0.0.1:9283
	CallbackSecret string // HMAC secret shared with the trust subject-kind registry
	// Site is kungal's catalog_site key. Used ONLY to scope the moderator inbox
	// proxy (Phase 3) so kungal moderators see kungal's review items, not other
	// sites'. Must match the oauth_clients.catalog_site binding; a wrong value
	// yields an empty (but safe) inbox.
	Site string
	// CheckEnabled / ScanEnabled are the TWO INDEPENDENT wave-1 moderation
	// switches (topic + reply create/edit), both default OFF. CheckEnabled gates
	// the SYNCHRONOUS pre-write word-list gate (deny blocks, hold publishes+logs,
	// fail-open); ScanEnabled gates the ASYNC post-commit shadow scan. Each is
	// keyed off its own env var, NEVER off client presence — so a reports-
	// configured production forum does not auto-enable check/scan on deploy. Both
	// additionally require the trust client to be configured (wired in app.go).
	CheckEnabled bool // KUN_TRUST_CHECK_ENABLED
	ScanEnabled  bool // KUN_TRUST_SCAN_ENABLED
}

// ArtifactClientConfig holds the credentials kungal uses to call the centralized
// artifact service (kun-galgame-infra :9279) for large-file (toolset archive)
// upload/download. Auth is HTTP Basic with an OAuth client_id/secret — the
// artifact service reuses the oauth_client table as its site registry (gated by
// artifact_enabled + artifact_site_key infra-side), so kungal's OAuth client IS
// its artifact site. ClientID/ClientSecret default to the OAuth credentials when
// unset (filled in app.go), so a single OAuth client works for both.
type ArtifactClientConfig struct {
	BaseURL      string // artifact service base, e.g. http://127.0.0.1:9279
	ClientID     string // OAuth client id (Basic auth); defaults to OAuth.ClientID
	ClientSecret string // OAuth client secret; defaults to OAuth.ClientSecret
}

// LinkCheckerConfig holds the s2s credentials kungal uses to call the
// kungal-link-live-checker service — the "report expired" gate that returns a
// conservative alive/dead/unknown verdict for a netdisk share link. When
// BaseURL/APIKey are unset the gate is skipped and a report falls back to the
// legacy single-report-expires behavior (see resource_service.MarkExpired).
type LinkCheckerConfig struct {
	BaseURL string // checker base, e.g. https://link-checker-kungal.nextmoe.dev
	APIKey  string // service Bearer key (one of the checker's LLC_API_KEYS)
	// Cloudflare Access service-token headers. The checker sits behind CF Access
	// (zero public anonymous exposure), so every request must carry
	// CF-Access-Client-Id / -Secret on top of the Bearer key. Leave both empty
	// when reaching a checker NOT behind CF Access (e.g. a local dev instance).
	CFAccessClientID     string
	CFAccessClientSecret string
}

// ImageClientConfig holds the credentials kungal uses to call the image
// service directly (multi-image upload paths for galgame covers /
// screenshots — U2). Distinct from NextMoeAPIConfig.ImageCDNBase which
// is just the public URL prefix for the response-rewrite walker.
//
// Set the three env vars below; the public CDN prefix is shared with
// the walker (NextMoeAPIConfig.ImageCDNBase) so it isn't duplicated.
// When ClientID/ClientSecret are unset, kungal's upload endpoints
// return a clear error (no silent fallback to a misconfigured client).
type ImageClientConfig struct {
	BaseURL      string // image service base, e.g. http://127.0.0.1:9278
	ClientID     string // OAuth client id (Basic auth)
	ClientSecret string // OAuth client secret
}

// NextMoeAPIConfig holds how kungal reaches the NextMoe catalog service's
// galgame surface. BaseURL is the HOST base (no /api or /internal suffix); the
// galgame client derives {base}/internal (internal-tier read face, X-API-Key)
// and {base}/api (legacy face for writes / admin / feeds). APIKey is the
// internal-tier devapi key sent as X-API-Key on read calls and both feeds. It
// is REQUIRED — Load() fail-fasts when a base is configured without a key;
// there is no keyless-fallback valve any more (wave 05).
type NextMoeAPIConfig struct {
	BaseURL string
	APIKey  string
	// ImageCDNBase is the image_service public CDN prefix (no trailing
	// slash), identical to the service's KUN_IMAGE_PUBLIC_BASE_URL. The
	// service returns image_service-backed banners as banner="" + a
	// hash; kungal resolves the hash → CDN URL server-side (in the galgame
	// client) so every downstream banner stays a plain usable URL. See
	// docs/galgame_wiki/07-submission.md §banner and
	// docs/image_service/06-integration-guide.md.
	ImageCDNBase string
}

// DlsiteConfig holds the DLsite affiliate wiring for the 补票 (buy-legit) prompt.
// kungal has an affiliate partnership; where a galgame resolves to a DLsite work
// number, the prompt offers a direct purchase link instead of only pointing at the
// 制作商 section.
//
// The whole link is assembled SERVER-side and shipped as a ready URL. Two reasons:
// the affiliate id / template stay out of the browser bundle, and this project's
// frontend build cannot be trusted with env vars (NUXT_PUBLIC_* / process.env.*
// come out undefined in the generic prod image — the same trap that bit the web
// build before), so a template baked into the frontend would silently produce
// broken links in production.
//
// LinkTemplate carries a `{workno}` placeholder and is deliberately a whole
// template rather than assembled parts: DLsite's affiliate path may differ per
// site segment (`/soft/` is the pro/VJ path; doujin/RJ may need another), so a
// path change stays an env edit instead of a code change.
type DlsiteConfig struct {
	// LinkTemplate is the per-work affiliate deep link, e.g.
	// https://dlaf.jp/soft/dlaf/=/t/s/link/work/aid/kungal/locale/zh_CN/id/{workno}.html/?locale=zh_CN
	// Empty = the feature is off and no link is ever emitted.
	LinkTemplate string
	// CouponURL is the partnership's coupon landing page. It MUST be a shortened
	// URL — the partner's requirement, so the raw domain does not get blocked by
	// network censorship. Empty = no coupon entry rendered.
	CouponURL string
}

// Configured reports whether per-work purchase links can be built.
func (c DlsiteConfig) Configured() bool { return c.LinkTemplate != "" }

type ServerConfig struct {
	Port string
	Mode string // "dev" or "prod"
}

type DatabaseConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int // seconds
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

type OAuthConfig struct {
	ServerURL    string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	JWTSecret    string
}

type S3Config struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
}

type MailConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
}

type SearchConfig struct {
	MeilisearchURL string
	MeilisearchKey string
}

type CORSConfig struct {
	AllowOrigins string
}

func Load() (*Config, error) {
	dbURL, err := requireEnv("KUN_DATABASE_URL")
	if err != nil {
		return nil, err
	}
	oauthServerURL, err := requireEnv("OAUTH_SERVER_URL")
	if err != nil {
		return nil, err
	}
	oauthClientID, err := requireEnv("OAUTH_CLIENT_ID")
	if err != nil {
		return nil, err
	}
	oauthClientSecret, err := requireEnv("OAUTH_CLIENT_SECRET")
	if err != nil {
		return nil, err
	}
	oauthRedirectURI, err := requireEnv("OAUTH_REDIRECT_URI")
	if err != nil {
		return nil, err
	}

	// The catalog galgame read face (the internal face) hard-depends on an
	// internal-tier API key — there is no keyless-fallback valve any more
	// (wave 05). A base configured without a key is a misconfiguration: fail
	// fast at startup, loudly naming the env var, rather than silently 401 on
	// every read at runtime. No silent degradation.
	nextMoeBase := envOrDefault("KUN_NEXTMOE_API_BASE", "http://127.0.0.1:19281")
	nextMoeKey := envOrDefault("KUN_NEXTMOE_API_KEY", "")
	if nextMoeBase != "" && nextMoeKey == "" {
		return nil, fmt.Errorf(
			"KUN_NEXTMOE_API_KEY 未设置: catalog galgame 读面 (internal 面) 硬依赖 internal-tier API key; 已配置 KUN_NEXTMOE_API_BASE=%q 但 KUN_NEXTMOE_API_KEY 为空 (keyless 回退阀已在 wave 05 移除, 不做静默降级)",
			nextMoeBase,
		)
	}

	return &Config{
		Server: ServerConfig{
			Port: envOrDefault("SERVER_PORT", "2334"),
			Mode: envOrDefault("SERVER_MODE", "dev"),
		},
		Database: DatabaseConfig{
			URL:             dbURL,
			MaxOpenConns:    envOrDefaultInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    envOrDefaultInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: envOrDefaultInt("DB_CONN_MAX_LIFETIME", 300),
		},
		Redis: RedisConfig{
			Host:     envOrDefault("REDIS_HOST", "127.0.0.1"),
			Port:     envOrDefault("REDIS_PORT", "6379"),
			Password: envOrDefault("REDIS_PASSWORD", ""),
			DB:       envOrDefaultInt("REDIS_DB", 0),
		},
		OAuth: OAuthConfig{
			ServerURL:    oauthServerURL,
			ClientID:     oauthClientID,
			ClientSecret: oauthClientSecret,
			RedirectURI:  oauthRedirectURI,
			JWTSecret:    envOrDefault("JWT_SECRET", ""),
		},
		// Archive uploads (Backblaze B2 in production): toolset .7z/.zip/.rar
		// files. This is the only S3-API bucket the forum still uses — inline /
		// content images all go through image_service now, not a local bucket.
		FileStorage: S3Config{
			Endpoint:  envOrDefault("FILE_STORAGE_ENDPOINT", ""),
			Region:    envOrDefault("FILE_STORAGE_REGION", ""),
			Bucket:    envOrDefault("FILE_STORAGE_BUCKET", ""),
			AccessKey: envOrDefault("FILE_STORAGE_ACCESS_KEY", ""),
			SecretKey: envOrDefault("FILE_STORAGE_SECRET_KEY", ""),
		},
		Mail: MailConfig{
			Host:     envOrDefault("MAIL_HOST", ""),
			Port:     envOrDefaultInt("MAIL_PORT", 587),
			User:     envOrDefault("MAIL_USER", ""),
			Password: envOrDefault("MAIL_PASSWORD", ""),
			From:     envOrDefault("MAIL_FROM", ""),
		},
		Search: SearchConfig{
			MeilisearchURL: envOrDefault("MEILISEARCH_URL", "http://127.0.0.1:7700"),
			MeilisearchKey: envOrDefault("MEILISEARCH_KEY", ""),
		},
		CORS: CORSConfig{
			AllowOrigins: envOrDefault(
				"CORS_ALLOW_ORIGINS",
				"http://127.0.0.1:2333,https://www.kungal.com",
			),
		},
		NextMoeAPI: NextMoeAPIConfig{
			// Host base, no /api suffix; the galgame client derives
			// {base}/internal (read face + feeds) + {base}/api (legacy face).
			// Dev default = local catalog dev instance on :19281.
			BaseURL: nextMoeBase,
			// Internal-tier devapi key (X-API-Key) for the read face + feeds.
			// REQUIRED — validated above (fail-fast; no keyless fallback).
			APIKey: nextMoeKey,
			// Must match the service's KUN_IMAGE_PUBLIC_BASE_URL exactly —
			// both build the same {base}/{hh}/{hh}/{hash}.webp layout.
			ImageCDNBase: envOrDefault("KUN_IMAGE_PUBLIC_BASE_URL", "https://image.kungal.iloveren.link"),
		},
		ImageClient: ImageClientConfig{
			BaseURL:      envOrDefault("KUN_IMAGE_CLIENT_BASE_URL", "http://127.0.0.1:9278"),
			ClientID:     envOrDefault("KUN_IMAGE_CLIENT_ID", ""),
			ClientSecret: envOrDefault("KUN_IMAGE_CLIENT_SECRET", ""),
		},
		ArtifactClient: ArtifactClientConfig{
			BaseURL:      envOrDefault("KUN_ARTIFACT_CLIENT_BASE_URL", "http://127.0.0.1:9279"),
			ClientID:     envOrDefault("KUN_ARTIFACT_CLIENT_ID", ""),
			ClientSecret: envOrDefault("KUN_ARTIFACT_CLIENT_SECRET", ""),
		},
		LinkChecker: LinkCheckerConfig{
			BaseURL:              envOrDefault("LINK_CHECKER_BASE_URL", ""),
			APIKey:               envOrDefault("LINK_CHECKER_API_KEY", ""),
			CFAccessClientID:     envOrDefault("CF_ACCESS_CLIENT_ID", ""),
			CFAccessClientSecret: envOrDefault("CF_ACCESS_CLIENT_SECRET", ""),
		},
		Trust: TrustConfig{
			BaseURL:        envOrDefault("KUN_TRUST_BASE_URL", "http://127.0.0.1:9283"),
			CallbackSecret: envOrDefault("KUN_TRUST_CALLBACK_SECRET", ""),
			Site:           envOrDefault("KUN_TRUST_SITE", "kungal"),
			CheckEnabled:   envOrDefaultBool("KUN_TRUST_CHECK_ENABLED", false),
			ScanEnabled:    envOrDefaultBool("KUN_TRUST_SCAN_ENABLED", false),
		},
		Catalog: CatalogClientConfig{
			BaseURL: envOrDefault("KUN_CATALOG_API_BASE", "http://127.0.0.1:9281"),
		},
		Community: CommunityConfig{
			// Empty base URL by default: the client's Configured() gate then
			// stays false on a dev box without a community service, so comment
			// reads degrade to empty pages and writes to 503.
			BaseURL:      envOrDefault("KUN_COMMUNITY_API_BASE", ""),
			ClientID:     envOrDefault("KUN_COMMUNITY_CLIENT_ID", ""),
			ClientSecret: envOrDefault("KUN_COMMUNITY_CLIENT_SECRET", ""),
		},
		Dlsite: DlsiteConfig{
			// Both empty by default: no template = no purchase link is emitted, so
			// an unconfigured deployment simply keeps today's 补票 prompt.
			LinkTemplate: envOrDefault("KUN_DLSITE_LINK_TEMPLATE", ""),
			CouponURL:    envOrDefault("KUN_DLSITE_COUPON_URL", ""),
		},
	}, nil
}

func requireEnv(key string) (string, error) {
	val := os.Getenv(key)
	if val == "" {
		return "", fmt.Errorf("环境变量 %s 未设置", key)
	}
	return val, nil
}

func envOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func envOrDefaultInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return fallback
}

func envOrDefaultBool(key string, fallback bool) bool {
	if val := os.Getenv(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return fallback
}
