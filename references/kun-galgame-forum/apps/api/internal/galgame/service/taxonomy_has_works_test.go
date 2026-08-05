package service

// The two taxonomy browse lanes ask for the NON-EMPTY vocabulary.
//
// Both indexes are mostly dead ends without it: 114 of 1,744 canonical tags
// have no works at all, and some 40% of the 37,623 labels are organisations
// nothing here credits — a browse page of "+ 0" cards that lead to an empty
// page. Upstream filters with the same predicate it counts with, so a listed
// row always has something to show and `total` converges with the rows.
//
// Pinned as an outgoing querystring because the failure is invisible from this
// side: drop the parameter and the pages still render, just full of dead ends
// again. The stub answers an empty page, which is all the two GetLists need —
// neither hydrates anything when there are no rows.

import (
	"context"
	"net/url"
	"testing"
)

func TestTagList_AsksForTagsThatHaveWorks(t *testing.T) {
	rec := &worksQueryRecorder{}
	svc := NewTagService(rec.client(t), &GalgameEnricher{}, nil)

	if _, appErr := svc.GetList(context.Background(), url.Values{}, true); appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if got := rec.get("has_works"); got != "1" {
		t.Fatalf("has_works = %q, want \"1\" — the empty vocabulary is back in the list", got)
	}
	// The age gate stays open on the identity lane; has_works is nsfw-aware and
	// reads the same population, so the two parameters must travel together.
	if got := rec.get("nsfw"); got != "1" {
		t.Fatalf("nsfw = %q, want \"1\"", got)
	}
}

func TestOfficialList_AsksForLabelsThatHaveWorks(t *testing.T) {
	rec := &worksQueryRecorder{}
	svc := NewOfficialService(rec.client(t), nil)

	if _, appErr := svc.GetList(context.Background(), url.Values{}); appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if got := rec.get("has_works"); got != "1" {
		t.Fatalf("has_works = %q, want \"1\" — the empty vocabulary is back in the list", got)
	}
	if got := rec.get("nsfw"); got != "1" {
		t.Fatalf("nsfw = %q, want \"1\"", got)
	}
}

// The kind filter is a separate axis and must survive alongside has_works —
// they are ANDed upstream, and a kind-filtered page that quietly lost its
// emptiness filter would look right while listing dead ends again.
func TestOfficialList_KindFilterKeepsHasWorks(t *testing.T) {
	rec := &worksQueryRecorder{}
	svc := NewOfficialService(rec.client(t), nil)

	_, appErr := svc.GetList(context.Background(), url.Values{"kind": {"game_brand"}})
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if got := rec.get("kind"); got != "game_brand" {
		t.Fatalf("kind = %q, want \"game_brand\"", got)
	}
	if got := rec.get("has_works"); got != "1" {
		t.Fatalf("has_works = %q, want \"1\"", got)
	}
}
