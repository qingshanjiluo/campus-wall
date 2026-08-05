package service

import (
	"reflect"
	"testing"
)

func sampleForm() *SubmissionForm {
	return &SubmissionForm{
		NameJaJP: "白恋サクラ", NameZhCN: "白恋樱", NameEnUS: "  ",
		IntroJaJP: "紹介", IntroZhCN: " ",
		ContentLimit: "nsfw", AgeLimit: "r18",
		OriginalLanguage: "ja-jp",
		Aliases:          []string{"シロコイ", "  "},
		ReleaseDate:      "2019-06-00",
	}
}

// The form speaks four product locales; the registry speaks BCP-47. A wrong
// mapping here does not fail — it files the title under a language nobody
// searches in.
func TestSubmissionFieldsTranslateLocales(t *testing.T) {
	fields := sampleForm().Fields()

	titles, ok := fields["catalog.work.titles"].([]any)
	if !ok {
		t.Fatalf("titles missing: %#v", fields["catalog.work.titles"])
	}
	want := []map[string]any{
		{"lang": "ja", "title": "白恋サクラ", "kind": titleKindOfficial},
		{"lang": "zh-Hans", "title": "白恋樱", "kind": titleKindOfficial},
		// An alias belongs to no language in particular, and the field accepts
		// the empty language for this kind alone.
		{"lang": "", "title": "シロコイ", "kind": titleKindAlias},
	}
	if len(titles) != len(want) {
		t.Fatalf("titles = %v, want %d entries (blank ones dropped, not sent empty)", titles, len(want))
	}
	for i, w := range want {
		if !reflect.DeepEqual(titles[i], any(w)) {
			t.Errorf("title %d = %v, want %v", i, titles[i], w)
		}
	}

	intros, _ := fields["catalog.work.intros"].([]any)
	if len(intros) != 1 || !reflect.DeepEqual(intros[0], any(map[string]any{"lang": "ja", "intro": "紹介"})) {
		t.Errorf("intros = %v, want only the non-blank ja one", intros)
	}
	if fields["catalog.work.olang"] != "ja" {
		t.Errorf("olang = %v, want ja (not the product locale ja-jp)", fields["catalog.work.olang"])
	}
	// The two editorial switches are DIFFERENT axes and must not collapse:
	// content_rating is what the game is rated, display_nsfw is whether the
	// material we would render is safe to show.
	if fields["catalog.work.content_rating"] != contentRatingR18 {
		t.Errorf("content_rating = %v, want r18", fields["catalog.work.content_rating"])
	}
	if fields["catalog.work.display_nsfw"] != true {
		t.Errorf("display_nsfw = %v, want true", fields["catalog.work.display_nsfw"])
	}
	// display_name is an identity, so it follows the form's own preference
	// order and never the reader's locale.
	if fields["catalog.work.display_name"] != "白恋サクラ" {
		t.Errorf("display_name = %v, want the ja title", fields["catalog.work.display_name"])
	}
	// Covers are not a submittable facet.
	if _, present := fields["catalog.work.covers"]; present {
		t.Error("covers must not ride the mint — the bytes are referenced, not carried")
	}
}

// An empty field is OMITTED, never sent blank: the mint reads a present key as
// an assertion, and an empty title list is refused outright.
func TestSubmissionOmitsEmptyLists(t *testing.T) {
	fields := (&SubmissionForm{NameJaJP: "x", AgeLimit: "all", ContentLimit: "sfw"}).Fields()
	if _, present := fields["catalog.work.intros"]; present {
		t.Error("an intro-less submission must omit the key, not send []")
	}
	if fields["catalog.work.content_rating"] != contentRatingAllAges {
		t.Errorf("content_rating = %v, want all_ages", fields["catalog.work.content_rating"])
	}
	if fields["catalog.work.display_nsfw"] != false {
		t.Errorf("display_nsfw = %v, want false", fields["catalog.work.display_nsfw"])
	}
}

// The nullable tail of the date IS the precision, so a truncated date is a
// legitimate value rather than an error — and a shape the three columns cannot
// express is refused rather than guessed.
func TestSubmissionReleaseDatePrecision(t *testing.T) {
	cases := []struct {
		raw     string
		want    *[3]int16
		wantErr bool
	}{
		{"", nil, false},                       // TBA
		{"2019", &[3]int16{2019, 0, 0}, false}, // year only
		{"2019-06-00", &[3]int16{2019, 6, 0}, false},
		{"2019-06-14", &[3]int16{2019, 6, 14}, false},
		{"2019-00-14", nil, true}, // a day with no month has no representation
		{"0000-01-01", nil, true},
		{"not-a-date", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			got, appErr := (&SubmissionForm{ReleaseDate: tc.raw}).Released()
			if tc.wantErr {
				if appErr == nil {
					t.Fatalf("want a refusal for %q, got %+v", tc.raw, got)
				}
				return
			}
			if appErr != nil {
				t.Fatalf("unexpected refusal: %v", appErr)
			}
			if tc.want == nil {
				if got != nil {
					t.Fatalf("want TBA (nil), got %+v", got)
				}
				return
			}
			if got == nil || got.Y != tc.want[0] || got.M != tc.want[1] || got.D != tc.want[2] {
				t.Errorf("got %+v, want %v", got, tc.want)
			}
		})
	}
}

// An unknown original-language code is passed through rather than dropped, so
// the registry's own whitelist answers with a 422 naming the field instead of
// the submission silently losing its language.
func TestSubmissionUnknownOLangIsPassedThrough(t *testing.T) {
	if got := olangOf("ko-kr"); got != "ko-kr" {
		t.Errorf("olangOf(ko-kr) = %q, want it forwarded for the registry to judge", got)
	}
	if got := olangOf("ZH-TW"); got != "zh-Hant" {
		t.Errorf("olangOf(ZH-TW) = %q, want zh-Hant", got)
	}
}
