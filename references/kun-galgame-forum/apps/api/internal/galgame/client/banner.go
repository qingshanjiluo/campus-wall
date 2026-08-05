package client

import (
	"bytes"
	"encoding/json"
	"strings"
)

// Banner / image resolution — server-side hash → CDN URL.
//
// Background: the Galgame Galgame is the SoT for galgame metadata. After
// galgame PR5 (K-PR6) the canonical banner source is the
// `covers[sort_order=0]` row — galgame exposes a derived `effective_banner_hash`
// on every detail / list / brief response (with hash + sort_order on
// each covers/screenshots row). The walker below turns hashes into
// ready-to-use CDN URLs in a single pass over the galgame response so
// downstream code never has to construct image URLs itself.
//
// URL layout mirrors the galgame's imageclient.MainURL exactly
// (kun-galgame-infra/apps/api/pkg/imageclient/client.go):
//
//	{cdnBase}/{hash[:2]}/{hash[2:4]}/{hash}.webp
//
// cdnBase MUST equal the galgame's KUN_IMAGE_PUBLIC_BASE_URL.

// bannerURLFromHash builds the main-image CDN URL for a content hash.
// Returns "" when the hash is too short to fan out (mirrors the galgame
// helper's len < 4 guard) so callers can fall back safely.
func bannerURLFromHash(cdnBase, hash string) string {
	if len(hash) < 4 {
		return ""
	}
	return strings.TrimRight(cdnBase, "/") + "/" +
		hash[:2] + "/" + hash[2:4] + "/" + hash + ".webp"
}

// rewriteBanners walks the galgame response JSON and, for every object
// carrying U2 hash fields (`effective_banner_hash` / per-row
// `image_hash` + `sort_order`), injects a CDN URL alongside (so the FE
// renders without computing URLs itself). See walkResolveBanner for
// the exact rules. The legacy `banner_image_hash → banner` fill rule
// was retired with galgame PR5 (K-PR6).
//
// Safety: any parse/encode failure returns the original bytes verbatim,
// so a malformed or unexpected payload can never be turned into an error
// by this cosmetic step. Numbers are decoded via json.Number so large
// IDs / timestamps round-trip with exact representation.
func rewriteBanners(raw json.RawMessage, cdnBase string) json.RawMessage {
	if cdnBase == "" || len(raw) == 0 {
		return raw
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return raw
	}

	if !walkResolveBanner(tree, cdnBase) {
		// Nothing rewritten — avoid a needless re-marshal (and any
		// incidental key reordering) when there was no work to do.
		return raw
	}

	out, err := json.Marshal(tree)
	if err != nil {
		return raw
	}
	return out
}

// walkResolveBanner recurses through maps/slices, resolving banners in
// place. Returns true if it changed anything.
//
// Two injection rules — both defensive (only fill empties; never
// overwrite an existing pre-resolved field):
//
//  1. (U2) Object with `effective_banner_hash` non-empty and no
//     `effective_banner_url` → fill `effective_banner_url`. This is the
//     forward-facing "head image" field (galgame detail / list cards).
//
//  2. (U2) Object with `image_hash` non-empty + `sort_order` present
//     and no `cdn_url` → fill `cdn_url`. Captures every row in a
//     `covers[]` / `screenshots[]` array — and any future image-bearing
//     relation — without the walker needing to know its parent key.
//     The sort_order guard distinguishes cover/screenshot rows from the
//     image_service bare image record (which carries image_hash but no
//     sort_order).
//
// Galgame PR5 (K-PR6) retired the legacy `banner_image_hash` → `banner`
// fill rule; the wire field is gone in both responses and snapshots,
// and `covers[sort_order=0]` is the canonical banner source.
//
// Rules are independent (an object MAY trigger both). Walking proceeds
// into all children regardless, so nested revision/PR snapshots are
// resolved in the same pass.
func walkResolveBanner(node any, cdnBase string) bool {
	changed := false
	switch v := node.(type) {
	case map[string]any:
		// Rule 1: effective_banner_hash → effective_banner_url (U2).
		if hashRaw, ok := v["effective_banner_hash"]; ok {
			hash, _ := hashRaw.(string)
			existing, _ := v["effective_banner_url"].(string)
			if strings.TrimSpace(hash) != "" && strings.TrimSpace(existing) == "" {
				if url := bannerURLFromHash(cdnBase, hash); url != "" {
					v["effective_banner_url"] = url
					changed = true
				}
			}
		}
		// Rule 2: image_hash (covers / screenshots / future) → cdn_url.
		// Skipped when the object is the GALGAME image-record itself (it has
		// its own `url` field) — the heuristic: cover/screenshot rows
		// also carry `sort_order`; image records do not. Cheap to guard.
		if hashRaw, ok := v["image_hash"]; ok {
			if _, isRow := v["sort_order"]; isRow {
				hash, _ := hashRaw.(string)
				existing, _ := v["cdn_url"].(string)
				if strings.TrimSpace(hash) != "" && strings.TrimSpace(existing) == "" {
					if url := bannerURLFromHash(cdnBase, hash); url != "" {
						v["cdn_url"] = url
						changed = true
					}
				}
			}
		}
		for _, child := range v {
			if walkResolveBanner(child, cdnBase) {
				changed = true
			}
		}
	case []any:
		for _, child := range v {
			if walkResolveBanner(child, cdnBase) {
				changed = true
			}
		}
	}
	return changed
}
