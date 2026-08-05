package gate

import (
	"context"
	"errors"
	"testing"
	"time"

	"kun-galgame-api/pkg/trustclient"
)

// fakeScanner records the last request and signals completion (ScanBg runs off a
// goroutine, so tests wait on done).
type fakeScanner struct {
	got  trustclient.ScanRequest
	err  error
	done chan struct{}
}

func newFakeScanner(err error) *fakeScanner {
	return &fakeScanner{err: err, done: make(chan struct{}, 1)}
}

func (f *fakeScanner) Scan(_ context.Context, req trustclient.ScanRequest) (*trustclient.ScanResult, error) {
	f.got = req
	f.done <- struct{}{}
	if f.err != nil {
		return nil, f.err
	}
	return &trustclient.ScanResult{ScanID: 1}, nil
}

func (f *fakeScanner) wait(t *testing.T) {
	t.Helper()
	select {
	case <-f.done:
	case <-time.After(2 * time.Second):
		t.Fatal("scan goroutine never fired")
	}
}

// A disabled sink (nil scanner) never dials.
func TestScanDisabledZeroDial(t *testing.T) {
	s := NewScanService(nil)
	if s.Enabled() {
		t.Fatal("nil scanner must be disabled")
	}
	s.ScanBg(SubjectKindTopic, "1", "x", 7) // must not panic / dial
	var nilSvc *ScanService
	nilSvc.ScanBg(SubjectKindReply, "1", "x", 7) // nil-safe
}

// The scan payload carries the registered kind, stringified id, RAW text, and
// author — proving text composition + subject identity reach the wire intact.
func TestScanBgPayload(t *testing.T) {
	fs := newFakeScanner(nil)
	s := NewScanService(fs)
	s.ScanBg(SubjectKindTopic, "8841", "标题\n\n正文", 99)
	fs.wait(t)
	if fs.got.SubjectKind != "forum_topic" || fs.got.SubjectID != "8841" {
		t.Fatalf("bad subject: %+v", fs.got)
	}
	if fs.got.Text != "标题\n\n正文" {
		t.Fatalf("bad text: %q", fs.got.Text)
	}
	if fs.got.AuthorID == nil || *fs.got.AuthorID != 99 {
		t.Fatalf("bad author: %+v", fs.got.AuthorID)
	}
}

// author_id <= 0 is omitted (optional field).
func TestScanBgOmitsZeroAuthor(t *testing.T) {
	fs := newFakeScanner(nil)
	NewScanService(fs).ScanBg(SubjectKindReply, "5", "正文", 0)
	fs.wait(t)
	if fs.got.AuthorID != nil {
		t.Fatalf("author 0 should be omitted, got %v", *fs.got.AuthorID)
	}
}

// A scan failure is best-effort: it logs and drops, never panics.
func TestScanBgFailureNoPanic(t *testing.T) {
	fs := newFakeScanner(errors.New("trust down"))
	NewScanService(fs).ScanBg(SubjectKindTopic, "1", "x", 1)
	fs.wait(t)
}
