package cron

import (
	"sort"
	"testing"
)

const (
	hashA = "7835f792543f8564cf95e7f84d4828f2a3ef735293f0844bf8ddf8f39371171d"
	hashB = "b380c8aad49b3c90029c92b36bcd32abcbc33011b1ad17053468037ad4304761"
)

func TestExtractContentImageHashes(t *testing.T) {
	contents := []string{
		// markdown image with a token + a title
		"看图 ![pic](/image/" + hashA + " \"标题\") 结束",
		// two tokens in one row, one of them a repeat of hashA (must dedupe)
		"a ![](/image/" + hashB + ") b ![](/image/" + hashA + ")",
		// absolute URL is NOT a token (must be ignored)
		"![](https://image.kungal.iloveren.link/78/35/" + hashA + ".webp)",
		// a /image/ path that is not a 64-hex token (public asset) — ignored
		"![](/image/kohaku.webp)",
		// uppercase hex — ignored (tokens are lowercase)
		"![](/image/" + "ABCDABCD" + ")",
		// no images at all
		"纯文本没有图片",
	}

	got := extractContentImageHashes(contents)
	sort.Strings(got)
	want := []string{hashA, hashB}
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("got %d distinct hashes %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("hash[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExtractContentImageHashes_Empty(t *testing.T) {
	if got := extractContentImageHashes(nil); len(got) != 0 {
		t.Errorf("nil input → %v, want empty", got)
	}
	if got := extractContentImageHashes([]string{"no tokens here"}); len(got) != 0 {
		t.Errorf("no-token input → %v, want empty", got)
	}
}

func TestQuoteIdent(t *testing.T) {
	cases := map[string]string{
		"content":              `"content"`,
		"last_message_content": `"last_message_content"`,
		"order":                `"order"`,                  // reserved word
		`we"ird`:               `"we""ird"`,                // embedded quote is doubled
	}
	for in, want := range cases {
		if got := quoteIdent(in); got != want {
			t.Errorf("quoteIdent(%q) = %q, want %q", in, got, want)
		}
	}
}
