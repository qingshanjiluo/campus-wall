package trustclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfigured(t *testing.T) {
	if New(Config{BaseURL: "http://x", ClientID: "", ClientSecret: ""}).Configured() {
		t.Fatal("expected not configured without credentials")
	}
	if New(Config{BaseURL: "", ClientID: "a", ClientSecret: "b"}).Configured() {
		t.Fatal("expected not configured without base URL")
	}
	if !New(Config{BaseURL: "http://x", ClientID: "a", ClientSecret: "b"}).Configured() {
		t.Fatal("expected configured with base URL + credentials")
	}
}

func TestSubmitReportNotConfigured(t *testing.T) {
	_, err := New(Config{}).SubmitReport(context.Background(), ReportRequest{})
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured, got %v", err)
	}
}

func TestSubmitReportSuccess(t *testing.T) {
	var gotAuth, gotPath string
	var gotBody ReportRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0,"message":"成功","data":{"report_id":42,"review_item_id":7}}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, ClientID: "cid", ClientSecret: "sec"})
	res, err := c.SubmitReport(context.Background(), ReportRequest{
		SubjectKind: "forum_topic", SubjectID: "1207", ReasonKey: "spam", ReporterID: 99,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ReportID != 42 || res.ReviewItemID != 7 {
		t.Fatalf("bad result: %+v", res)
	}
	if gotAuth == "" || gotAuth[:6] != "Basic " {
		t.Fatalf("expected Basic auth, got %q", gotAuth)
	}
	if gotPath != "/api/v1/trust/reports" {
		t.Fatalf("bad path %q", gotPath)
	}
	if gotBody.SubjectKind != "forum_topic" || gotBody.SubjectID != "1207" || gotBody.ReporterID != 99 {
		t.Fatalf("bad forwarded body: %+v", gotBody)
	}
}

func TestListReportReasons(t *testing.T) {
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"code":0,"message":"成功","data":{"reasons":[{"id":1,"key":"spam","name_cn":"垃圾信息","severity":1,"is_deprecated":false}]}}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, ClientID: "cid", ClientSecret: "sec"})
	reasons, err := c.ListReportReasons(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reasons) != 1 || reasons[0].Key != "spam" || reasons[0].NameCN != "垃圾信息" {
		t.Fatalf("bad reasons: %+v", reasons)
	}
	if gotPath != "/api/v1/trust/report-reasons" {
		t.Fatalf("bad path %q", gotPath)
	}
	if gotAuth[:6] != "Basic " {
		t.Fatalf("expected Basic auth, got %q", gotAuth)
	}

	// Not configured → error (service falls back to the local constant).
	if _, err := New(Config{}).ListReportReasons(context.Background()); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured, got %v", err)
	}
}

func TestSubmitReportErrorMapping(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   error
	}{
		{http.StatusTooManyRequests, `{"code":10,"message":"rate"}`, ErrRateLimited},
		{http.StatusUnprocessableEntity, `{"code":7,"message":"bad kind"}`, ErrValidation},
		{http.StatusForbidden, `{"code":5,"message":"no site"}`, ErrForbidden},
		{http.StatusUnauthorized, `{"code":10001,"message":"bad creds"}`, ErrUnauthorized},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		c := New(Config{BaseURL: srv.URL, ClientID: "cid", ClientSecret: "sec"})
		_, err := c.SubmitReport(context.Background(), ReportRequest{SubjectKind: "x", SubjectID: "1", ReasonKey: "spam", ReporterID: 1})
		if !errors.Is(err, tc.want) {
			t.Fatalf("status %d: want %v, got %v", tc.status, tc.want, err)
		}
		srv.Close()
	}
}
