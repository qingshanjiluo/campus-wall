package trustclient

import (
	"strconv"
	"testing"
	"time"
)

func TestVerifyCallbackSignature(t *testing.T) {
	secret := "s3cr3t"
	now := time.Unix(1_700_000_000, 0)
	ts := strconv.FormatInt(now.Unix(), 10)
	body := []byte(`{"disposition_id":42,"subject_kind":"forum_topic","subject_id":"1","action":1,"reason_code":"spam"}`)
	sig := SignPayload(secret, ts, body)

	if !VerifyCallbackSignature(secret, ts, sig, body, now) {
		t.Fatal("valid signature within window should verify")
	}
	if VerifyCallbackSignature("", ts, sig, body, now) {
		t.Fatal("empty secret must fail closed")
	}
	if VerifyCallbackSignature(secret, ts, sig+"00", body, now) {
		t.Fatal("tampered signature must fail")
	}
	if VerifyCallbackSignature(secret, ts, sig, append(body, '!'), now) {
		t.Fatal("tampered body must fail")
	}
	// Timestamp 6 minutes old → outside ±5min window.
	if VerifyCallbackSignature(secret, ts, sig, body, now.Add(6*time.Minute)) {
		t.Fatal("stale timestamp must fail")
	}
	// A different secret must not verify.
	if VerifyCallbackSignature("other", ts, sig, body, now) {
		t.Fatal("wrong secret must fail")
	}
}
