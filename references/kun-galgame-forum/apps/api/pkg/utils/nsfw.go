package utils

import (
	"encoding/json"
	"net/url"

	"github.com/gofiber/fiber/v3"
)

// IsSFW reads the Pinia persisted settings cookie and returns
// whether the user has NSFW content disabled (default: true/SFW).
//
// The preference vocabulary is sfw | nsfw | all, and BOTH `nsfw` and `all`
// mean NSFW-enabled — `all` is the older "show everything" value, still
// persisted in the cookie jar of anyone who set it before the sidebar toggle
// narrowed to two values. It used to fall through to SFW here while every
// frontend reader (pages/galgame/[gid]/index.vue, components/galgame/Tag.vue,
// the sidebar toggle) counted it as NSFW-on, so one setting meant two opposite
// things on the two sides of the wire: the FE unhid adult tag chips the server
// had never sent. The two sides now agree.
func IsSFW(c fiber.Ctx) bool {
	raw := c.Cookies("KUNGalgameSettings", "")
	if raw == "" {
		return true
	}

	// Pinia persisted state may URL-encode the cookie value
	decoded, err := url.QueryUnescape(raw)
	if err != nil {
		decoded = raw
	}

	var settings struct {
		ShowKUNGalgameContentLimit string `json:"showKUNGalgameContentLimit"`
	}
	if err := json.Unmarshal([]byte(decoded), &settings); err != nil {
		return true
	}

	return settings.ShowKUNGalgameContentLimit != "nsfw" &&
		settings.ShowKUNGalgameContentLimit != "all"
}
