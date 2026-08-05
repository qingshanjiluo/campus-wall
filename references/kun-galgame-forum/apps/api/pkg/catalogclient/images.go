package catalogclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
)

// images.go — the edit face's byte-upload leg (POST /api/v1/catalog/edit/images,
// infra wave 169). Covers/screenshots are edited as HASH ROWS on the proposal
// face; this call is where the hash comes from: the catalog uploads the bytes
// under its own image-service identity (site "catalog", kept alive by its
// refping cron) and hands the hash back for the edit payload to carry.

// EditImageResult is the image-service upload result the catalog forwards.
type EditImageResult struct {
	Hash         string            `json:"hash"`
	URL          string            `json:"url"`
	VariantURLs  map[string]string `json:"variant_urls"`
	Width        int               `json:"width"`
	Height       int               `json:"height"`
	Thumbhash    string            `json:"thumbhash"`
	SizeBytes    int64             `json:"size_bytes"`
	Deduplicated bool              `json:"deduplicated"`
}

// UploadEditImage forwards one cover/screenshot to the catalog edit face.
// preset is the image-service vocabulary the FE already speaks:
// "galgame_banner" (cover) or "galgame_screenshot". actorUID is the asserted
// end user, stamped into the image audit trail upstream.
//
// A non-2xx with a readable envelope surfaces as *EditAPIError so the handler
// can show the upstream message (quota, moderation) instead of a generic 500.
func (c *Client) UploadEditImage(ctx context.Context, r io.Reader, filename, preset string, actorUID int) (*EditImageResult, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("build multipart: %w", err)
	}
	if _, err := io.Copy(fw, r); err != nil {
		return nil, fmt.Errorf("copy file: %w", err)
	}
	if err := mw.WriteField("preset", preset); err != nil {
		return nil, err
	}
	if actorUID > 0 {
		if err := mw.WriteField("actor_uid", strconv.Itoa(actorUID)); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+editBase+"/images", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.basicAuth)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, ErrUnauthorized
		}
		return nil, fmt.Errorf("%w (status %d): malformed envelope", ErrUpstream, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK || env.Code != 0 {
		return nil, &EditAPIError{Status: resp.StatusCode, Code: env.Code, Message: env.Message}
	}
	var out EditImageResult
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return nil, fmt.Errorf("%w: malformed upload result", ErrUpstream)
	}
	return &out, nil
}
