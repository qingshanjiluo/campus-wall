package service

import (
	"strconv"
	"strings"

	"kun-galgame-api/pkg/catalogclient"
	"kun-galgame-api/pkg/errors"
)

// The submission form's payload, translated onto the registry's field keys.
//
// kungal's form speaks the four product locales and two editorial switches; the
// registry speaks BCP-47 languages, a rating enum and a display flag. The
// translation lives here, in ONE direction, rather than being spread across the
// handler and the client: the same four keys are read by the wizard, the draft
// editor and the schema-driven editor, and three copies of "zh-cn means
// zh-Hans" is how one of them ends up disagreeing.
//
// What the form loses, and why it is not smuggled through anyway:
//
//   - the banner. Covers are not a submittable facet — the registry takes field
//     values, and a cover is a reference to bytes that must already exist in the
//     image service. It rides a follow-up edit instead (see SubmitCoverPatch).
//   - the release-date PRECISION enum. The registry expresses precision as the
//     nullable tail of the date, so "2019" is {y:2019} rather than a date plus a
//     word claiming how much of it to believe.

// submissionLocales maps kungal's product locales onto the registry's languages,
// in the order a title list should carry them.
var submissionLocales = []struct{ product, lang string }{
	{"ja-jp", "ja"},
	{"zh-cn", "zh-Hans"},
	{"zh-tw", "zh-Hant"},
	{"en-us", "en"},
}

// olangOf translates the form's original-language code. The form offers the
// four product locales; anything else came from a wiki-era value and is passed
// through lowercased, where the registry's own whitelist judges it (a 422 that
// names the field, not a silently dropped language).
func olangOf(productCode string) string {
	// Locale codes are case-insensitive on the wire and the form has shipped
	// both spellings; matching case-sensitively would send `zh-TW` through as an
	// unrecognized language instead of translating it.
	code := strings.ToLower(strings.TrimSpace(productCode))
	for _, l := range submissionLocales {
		if l.product == code {
			return l.lang
		}
	}
	return code
}

// Content-rating values (registry enum). The form has a two-way age switch, so
// only two of the three are reachable from it; `sensitive` exists on the axis
// and is set by curation.
const (
	contentRatingAllAges = 0
	contentRatingR18     = 2
)

// Title kinds (registry enum).
const (
	titleKindOfficial = 0
	titleKindAlias    = 1
)

// SubmissionForm is the wizard's payload in the forum's own vocabulary. It is
// deliberately NOT the registry's shape: the form is a product surface and gets
// to keep its product locales, while everything registry-shaped is derived by
// Fields below.
type SubmissionForm struct {
	NameEnUS string `json:"name_en_us"`
	NameJaJP string `json:"name_ja_jp"`
	NameZhCN string `json:"name_zh_cn"`
	NameZhTW string `json:"name_zh_tw"`

	IntroEnUS string `json:"intro_en_us"`
	IntroJaJP string `json:"intro_ja_jp"`
	IntroZhCN string `json:"intro_zh_cn"`
	IntroZhTW string `json:"intro_zh_tw"`

	ContentLimit     string   `json:"content_limit"`
	AgeLimit         string   `json:"age_limit"`
	OriginalLanguage string   `json:"original_language"`
	Aliases          []string `json:"aliases"`

	// ReleaseDate is "" (unknown / TBA) or YYYY-MM-DD, optionally with a zero
	// month or day standing for "unknown at that level" — which is how the form
	// has always let a submitter say "2019, month unknown".
	ReleaseDate string `json:"release_date"`
	// BannerHash is the already-uploaded banner's image hash, or "". It never
	// reaches the mint; see SubmitCoverPatch.
	BannerHash string `json:"banner_hash"`
}

func (f *SubmissionForm) names() map[string]string {
	return map[string]string{
		"ja-jp": f.NameJaJP, "zh-cn": f.NameZhCN,
		"zh-tw": f.NameZhTW, "en-us": f.NameEnUS,
	}
}

func (f *SubmissionForm) intros() map[string]string {
	return map[string]string{
		"ja-jp": f.IntroJaJP, "zh-cn": f.IntroZhCN,
		"zh-tw": f.IntroZhTW, "en-us": f.IntroEnUS,
	}
}

// DisplayName is the entry's one canonical label. It follows the form's own
// locale preference order rather than the reader's: the registry's display_name
// is an identity, not a rendering, and it must not depend on who submitted.
func (f *SubmissionForm) DisplayName() string {
	names := f.names()
	for _, l := range submissionLocales {
		if v := strings.TrimSpace(names[l.product]); v != "" {
			return v
		}
	}
	return ""
}

// Fields renders the form onto the registry's submission field keys. Empty
// values are OMITTED rather than sent blank — the mint treats a present key as
// an assertion, and an empty title list is refused outright.
func (f *SubmissionForm) Fields() map[string]any {
	fields := map[string]any{
		"catalog.work.display_name": f.DisplayName(),
		"catalog.work.olang":        olangOf(f.OriginalLanguage),
		"catalog.work.display_nsfw": f.ContentLimit == "nsfw",
	}
	rating := contentRatingAllAges
	if f.AgeLimit == "r18" {
		rating = contentRatingR18
	}
	fields["catalog.work.content_rating"] = rating

	names := f.names()
	titles := make([]any, 0, len(submissionLocales)+len(f.Aliases))
	for _, l := range submissionLocales {
		if v := strings.TrimSpace(names[l.product]); v != "" {
			titles = append(titles, map[string]any{"lang": l.lang, "title": v, "kind": titleKindOfficial})
		}
	}
	// Aliases carry NO language — an alias is a string people also call the
	// work by and belongs to no language in particular. The field accepts the
	// empty language for this kind alone.
	for _, alias := range f.Aliases {
		if v := strings.TrimSpace(alias); v != "" {
			titles = append(titles, map[string]any{"lang": "", "title": v, "kind": titleKindAlias})
		}
	}
	if len(titles) > 0 {
		fields["catalog.work.titles"] = titles
	}

	intros := f.intros()
	introList := make([]any, 0, len(submissionLocales))
	for _, l := range submissionLocales {
		if v := strings.TrimSpace(intros[l.product]); v != "" {
			introList = append(introList, map[string]any{"lang": l.lang, "intro": v})
		}
	}
	if len(introList) > 0 {
		fields["catalog.work.intros"] = introList
	}
	return fields
}

// Released parses the form's date into the registry's fuzzy one. nil = TBA.
//
// The nullable tail IS the precision, so a zero month or day simply stops the
// date there; a day without a month is refused rather than guessed, because
// there is no shape that expresses it.
func (f *SubmissionForm) Released() (*catalogclient.WorkSubmitDate, *errors.AppError) {
	raw := strings.TrimSpace(f.ReleaseDate)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, "-")
	if len(parts) == 0 || len(parts) > 3 {
		return nil, errors.ErrValidation("发售日期格式应为 YYYY-MM-DD")
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return nil, errors.ErrValidation("发售日期格式应为 YYYY-MM-DD")
		}
		nums[i] = n
	}
	if nums[0] <= 0 {
		return nil, errors.ErrValidation("发售日期需要年份")
	}
	if nums[1] == 0 && nums[2] != 0 {
		return nil, errors.ErrValidation("发售日期不能只有日而没有月")
	}
	return &catalogclient.WorkSubmitDate{
		Y: int16(nums[0]), M: int16(nums[1]), D: int16(nums[2]),
	}, nil
}

// CoverPatch is the follow-up edit that attaches the submitted banner, or nil
// when none was uploaded. Covers are excluded from the mint on purpose (the
// bytes must exist before anything may reference them), so the banner becomes
// the submission's first EDIT — visible to the reviewer alongside it, and
// subject to the same field rules as any other cover change.
func (f *SubmissionForm) CoverPatch() map[string]any {
	if strings.TrimSpace(f.BannerHash) == "" {
		return nil
	}
	return map[string]any{
		"catalog.work.covers": []any{
			map[string]any{
				"image_hash": f.BannerHash, "sort_order": 0,
				"sexual": 0, "violence": 0, "source": "", "source_key": "", "kind": "",
			},
		},
	}
}
