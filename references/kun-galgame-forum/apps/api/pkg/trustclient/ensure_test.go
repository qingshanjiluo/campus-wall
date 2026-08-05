package trustclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEnsureSubjectKindsNotConfigured(t *testing.T) {
	_, err := New(Config{}).EnsureSubjectKinds(context.Background(), []EnsureSubjectKindItem{{Key: "forum_topic"}})
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured, got %v", err)
	}
}

func TestEnsureSubjectKindsSuccess(t *testing.T) {
	var gotAuth, gotPath, gotContentType string
	var gotBody struct {
		Kinds []EnsureSubjectKindItem `json:"kinds"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0,"message":"成功","data":{"results":[` +
			`{"key":"forum_topic","result":"created"},` +
			`{"key":"forum_reply","result":"unchanged"}` +
			`]}}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, ClientID: "cid", ClientSecret: "sec"})
	results, err := c.EnsureSubjectKinds(context.Background(), []EnsureSubjectKindItem{
		{Key: "forum_topic"}, {Key: "forum_reply"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Path + auth + content-type.
	if gotPath != "/api/v1/trust/subject-kinds/ensure" {
		t.Fatalf("bad path %q", gotPath)
	}
	if len(gotAuth) < 6 || gotAuth[:6] != "Basic " {
		t.Fatalf("expected Basic auth, got %q", gotAuth)
	}
	if gotContentType != "application/json" {
		t.Fatalf("expected JSON content-type, got %q", gotContentType)
	}

	// Body shape: {"kinds":[{"key":...}]} with keys only.
	if len(gotBody.Kinds) != 2 || gotBody.Kinds[0].Key != "forum_topic" || gotBody.Kinds[1].Key != "forum_reply" {
		t.Fatalf("bad forwarded body: %+v", gotBody.Kinds)
	}

	// Result decoding, in request order.
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	if results[0].Key != "forum_topic" || results[0].Result != "created" {
		t.Fatalf("bad result[0]: %+v", results[0])
	}
	if results[1].Key != "forum_reply" || results[1].Result != "unchanged" {
		t.Fatalf("bad result[1]: %+v", results[1])
	}
}
