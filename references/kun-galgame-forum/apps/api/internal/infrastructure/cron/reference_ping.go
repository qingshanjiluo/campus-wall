package cron

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"kun-galgame-api/pkg/imageclient"

	"gorm.io/gorm"
)

// contentImageHashRe extracts the 64-hex hash from a /image/<hash> content token.
var contentImageHashRe = regexp.MustCompile(`/image/([0-9a-f]{64})`)

// refPingBatchSize caps each /image/reference-ping call (image_service accepts
// ≤1000 hashes per batch — docs/image_service/03-api-design.md).
const refPingBatchSize = 1000

// ErrRefPingNoEffect signals that content references image hashes but
// image_service updated NONE of them — the silent-breakage shape (wrong
// client/site, or the service is down). This is exactly what left the galgame
// images on the GC clock undetected; surfacing it as an error makes a daily
// no-op ping fail LOUDLY instead of quietly reporting "0 updated".
var ErrRefPingNoEffect = errors.New(
	"内容图 reference-ping 命中 0 个 hash (内容里有 token 但全部 not_found) — 疑似 image client/site 配错或服务异常",
)

// RunReferencePing refreshes last_referenced_at for every image_service hash the
// forum references in content, so image-gc never reclaims a live content image
// (TTL-driven: a hash >365d without a ping is soft-deleted, +30d physically —
// docs/image_service/06-integration-guide.md §保活).
//
// SCHEMA-DERIVED scope: rather than hardcode a column list — which drifted out of
// sync with the data three times (a migrated column not added to the ping) — we
// enumerate EVERY text column from information_schema and scan each for
// /image/<hash> tokens. New token-bearing columns are kept alive automatically;
// pinging a stray non-token hash is harmless (image_service returns not_found).
// (Forum stores no local *_image_hash columns: avatars are OAuth's [site=account],
// galgame cover/screenshot hashes are galgame's, pinged by infra's galgame refping.
// The forum client is site=kungal — the same site these images were uploaded
// under — so the site-scoped pings hit.)
//
// Returns (distinct hashes seen, hashes image_service updated). A failed column
// scan or ping batch is logged and the rest still run. Returns ErrRefPingNoEffect
// when there ARE hashes but NONE updated — the loud signal of silent breakage.
func RunReferencePing(
	ctx context.Context, db *gorm.DB, imgCli *imageclient.Client,
) (distinct int, updated int64, err error) {
	if imgCli == nil {
		return 0, 0, nil
	}

	hashes, err := collectContentImageHashes(ctx, db)
	if err != nil {
		return 0, 0, err
	}

	for start := 0; start < len(hashes); start += refPingBatchSize {
		end := min(start+refPingBatchSize, len(hashes))
		res, e := imgCli.ReferencePing(ctx, hashes[start:end])
		if e != nil {
			slog.Error("内容图 reference-ping 批次失败", "from", start, "to", end, "error", e)
			if err == nil {
				err = e
			}
			continue
		}
		updated += res.Updated
	}

	// Loud-fail guard: content references hashes but image_service updated none.
	if err == nil && len(hashes) > 0 && updated == 0 {
		err = ErrRefPingNoEffect
	}
	return len(hashes), updated, err
}

// collectContentImageHashes enumerates every text column in the public schema and
// extracts the distinct /image/<hash> tokens across all of them. Deriving the
// column set from the schema (not a hardcoded list) is the whole point — it can't
// drift out of sync with the columns that actually hold tokens.
func collectContentImageHashes(ctx context.Context, db *gorm.DB) ([]string, error) {
	type column struct {
		TableName  string
		ColumnName string
	}
	var cols []column
	if err := db.WithContext(ctx).Raw(`
		SELECT table_name, column_name FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND data_type IN ('text', 'character varying', 'character')
	`).Scan(&cols).Error; err != nil {
		return nil, err
	}

	type contentRow struct{ Content string }
	var contents []string
	for _, c := range cols {
		col := quoteIdent(c.ColumnName)
		// The %/image/% pre-filter keeps this cheap: only token-bearing rows are
		// read, so the 100+ columns with no tokens cost just a count-style scan.
		q := fmt.Sprintf(
			"SELECT %s AS content FROM %s WHERE %s LIKE '%%/image/%%'",
			col, quoteIdent(c.TableName), col,
		)
		var rows []contentRow
		if err := db.WithContext(ctx).Raw(q).Scan(&rows).Error; err != nil {
			// One odd column shouldn't sink the whole ping.
			slog.Warn("reference-ping 扫描列失败, 跳过",
				"table", c.TableName, "column", c.ColumnName, "error", err)
			continue
		}
		for _, r := range rows {
			contents = append(contents, r.Content)
		}
	}
	return extractContentImageHashes(contents), nil
}

// quoteIdent double-quotes a SQL identifier (table/column from information_schema)
// so an odd name (reserved word, mixed case) is referenced safely.
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// extractContentImageHashes pulls every distinct /image/<hash> token hash out of
// a set of content strings. A row may carry several images (and the same image
// may repeat across rows), so the result is de-duplicated.
func extractContentImageHashes(contents []string) []string {
	seen := make(map[string]struct{})
	for _, c := range contents {
		for _, m := range contentImageHashRe.FindAllStringSubmatch(c, -1) {
			seen[m[1]] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for h := range seen {
		out = append(out, h)
	}
	return out
}
