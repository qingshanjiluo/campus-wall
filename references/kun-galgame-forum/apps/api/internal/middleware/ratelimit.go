package middleware

import (
	"fmt"
	"time"

	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/response"

	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
)

// rateLimitScript atomically increments the per-user counter and guarantees it
// always carries a TTL. A plain INCR-then-EXPIRE is two round-trips: if the
// process dies (or the EXPIRE is dropped) in between, the key lingers with no
// expiry and locks the user out forever. Running both in one Lua script closes
// that window; re-arming the TTL whenever it is missing (PTTL < 0 — key exists
// but has no expiry) also self-heals any key already stranded without one. The
// TTL is set only when absent, so a fixed window resets cleanly rather than
// sliding forward on every hit. Returns the post-increment count.
var rateLimitScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if redis.call('PTTL', KEYS[1]) < 0 then
	redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return count
`)

// RateLimit creates a per-user rate limiting middleware using Redis.
// maxRequests is the maximum number of requests allowed within the window.
func RateLimit(rdb *redis.Client, prefix string, maxRequests int, window time.Duration) fiber.Handler {
	return func(c fiber.Ctx) error {
		user := GetUser(c)
		if user == nil {
			return response.Error(c, errors.ErrAuthExpired())
		}

		key := fmt.Sprintf("ratelimit:%s:%d", prefix, user.ID)
		count, err := rateLimitScript.Run(
			c.Context(), rdb, []string{key}, window.Milliseconds(),
		).Int64()
		if err != nil {
			// Fail open: a Redis blip must not take down the write path.
			return c.Next()
		}

		if count > int64(maxRequests) {
			return response.Error(c, errors.ErrBadRequest("操作过于频繁，请稍后再试"))
		}

		return c.Next()
	}
}
