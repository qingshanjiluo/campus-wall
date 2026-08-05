package service

import (
	"testing"

	"kun-galgame-api/internal/galgame/dto"
)

// Regression guard: covers/screenshots MUST carry the rewriteBanners-injected
// `cdn_url` from the galgame parse struct through to the FE DTO. It was silently
// dropped once — NextMoeGalgame{Cover,Screenshot} lacked the CDNURL field AND the
// mappers didn't copy it — so the detail gallery reached the FE with no image
// URLs and rendered nothing (falling back to a /image/<hash> redirect per
// image). The bytes DO carry cdn_url (the client runs rewriteBanners before we
// unmarshal); this asserts the typed detail path preserves it.
func TestCoversFromNextMoe_CarriesCDNURL(t *testing.T) {
	in := []dto.NextMoeGalgameCover{
		{ImageHash: "h0", SortOrder: 0, Sexual: 1, Violence: 2,
			Source: "vndb", SourceKey: "k0", CDNURL: "https://cdn/h0/aa/h0.webp"},
		{ImageHash: "h1", SortOrder: 1, CDNURL: "https://cdn/h1/aa/h1.webp"},
	}
	out := coversFromNextMoe(in)
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
	if out[0].CDNURL != "https://cdn/h0/aa/h0.webp" ||
		out[1].CDNURL != "https://cdn/h1/aa/h1.webp" {
		t.Errorf("cdn_url not carried: %q, %q", out[0].CDNURL, out[1].CDNURL)
	}
	if out[0].ImageHash != "h0" || out[0].Sexual != 1 || out[0].SourceKey != "k0" {
		t.Errorf("scalar fields mismapped: %+v", out[0])
	}
}

func TestScreenshotsFromNextMoe_CarriesCDNURL(t *testing.T) {
	in := []dto.NextMoeGalgameScreenshot{
		{ImageHash: "h0", SortOrder: 0, Caption: "cap",
			CDNURL: "https://cdn/h0/aa/h0.webp"},
	}
	out := screenshotsFromNextMoe(in)
	if len(out) != 1 {
		t.Fatalf("len = %d, want 1", len(out))
	}
	if out[0].CDNURL != "https://cdn/h0/aa/h0.webp" {
		t.Errorf("cdn_url not carried: %q", out[0].CDNURL)
	}
	if out[0].Caption != "cap" {
		t.Errorf("caption mismapped: %q", out[0].Caption)
	}
}
