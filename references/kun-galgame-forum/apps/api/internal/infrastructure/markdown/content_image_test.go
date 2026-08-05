package markdown

import (
	"strings"
	"testing"

	"kun-galgame-api/pkg/imageclient"
)

// A real 64-hex content hash (from prod): aa=78, bb=35.
const testHash = "7835f792543f8564cf95e7f84d4828f2a3ef735293f0844bf8ddf8f39371171d"

func TestResolveContentImageRef(t *testing.T) {
	// image.kungal.iloveren.link is what the inline allow-list accepts, so the
	// same base works for both the unit cases here and the RenderInline test.
	SetContentImageCDNBase("https://image.kungal.iloveren.link/")
	defer SetContentImageCDNBase("")

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"main", "/image/" + testHash,
			"https://image.kungal.iloveren.link/78/35/" + testHash + ".webp"},
		{"variant", "/image/" + testHash + "_256",
			"https://image.kungal.iloveren.link/78/35/" + testHash + "_256.webp"},
		{"absolute url is not a token", "https://image.kungal.iloveren.link/78/35/" + testHash + ".webp", ""},
		{"external url untouched", "https://example.com/a.png", ""},
		{"too short", "/image/abc", ""},
		{"trailing junk", "/image/" + testHash + "x", ""},
		{"uppercase hex rejected", "/image/" + strings.ToUpper(testHash), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveContentImageRef(c.in); got != c.want {
				t.Errorf("resolveContentImageRef(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// The content-image meta transformer must stamp width/height (aspect-ratio
// reservation, no CLS) + data-thumbhash (blur-up) onto the rendered <img>, and
// the sanitizer must keep them. A hash the resolver doesn't know stays a plain
// <img> (graceful — e.g. before the image_service thumbhash backfill runs).
func TestRenderInjectsContentImageMeta(t *testing.T) {
	SetContentImageCDNBase("https://image.kungal.iloveren.link")
	defer SetContentImageCDNBase("")

	SetContentImageMetaResolver(func(hashes []string) map[string]imageclient.ImageMeta {
		out := map[string]imageclient.ImageMeta{}
		for _, h := range hashes {
			if h == testHash {
				out[h] = imageclient.ImageMeta{Width: 1280, Height: 720, Thumbhash: "1QcSHQRnh493V4dIh4eXh1h4kJUI"}
			}
		}
		return out
	})
	defer SetContentImageMetaResolver(nil)

	out := Render("![pic](/image/" + testHash + ")")
	for _, want := range []string{`width="1280"`, `height="720"`, `data-thumbhash="1QcSHQRnh493V4dIh4eXh1h4kJUI"`} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in output\n got: %s", want, out)
		}
	}

	// An unknown hash (resolver returns nothing) must NOT gain meta attrs.
	const otherHash = "0000000000000000000000000000000000000000000000000000000000000000"
	if got := Render("![x](/image/" + otherHash + ")"); strings.Contains(got, "width=") {
		t.Errorf("unknown hash must not gain width attr\n got: %s", got)
	}
}

func TestResolveContentImageRef_EmptyBaseDisables(t *testing.T) {
	SetContentImageCDNBase("")
	if got := resolveContentImageRef("/image/" + testHash); got != "" {
		t.Errorf("empty base must disable resolution, got %q", got)
	}
}

// The token must be resolved to the absolute CDN URL in BOTH the block (topic)
// and inline (chat/PM) renderers — and in the inline case, resolving BEFORE
// sanitization is what lets it survive the strict img-src host allow-list.
func TestRenderResolvesContentImageToken(t *testing.T) {
	SetContentImageCDNBase("https://image.kungal.iloveren.link")
	defer SetContentImageCDNBase("")

	wantSrc := "https://image.kungal.iloveren.link/78/35/" + testHash + ".webp"
	md := "![pic](/image/" + testHash + ")"

	for _, r := range []struct {
		name string
		fn   func(string) string
	}{
		{"Render", Render},
		{"RenderInline", RenderInline},
	} {
		t.Run(r.name, func(t *testing.T) {
			out := r.fn(md)
			if !strings.Contains(out, wantSrc) {
				t.Errorf("%s: want resolved src %q in output\n got: %s", r.name, wantSrc, out)
			}
			// The raw token must NOT survive into the rendered HTML.
			if strings.Contains(out, `src="/image/`) {
				t.Errorf("%s: raw /image/<hash> token leaked unresolved\n got: %s", r.name, out)
			}
		})
	}
}
