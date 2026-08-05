package service

// staleCache is the read-through cache the precomputed browse indexes share.
//
// Both indexes have the same shape of problem: the catalog's browse lanes are
// keyset-paged and carry filters this product cannot express upstream, so an
// honest page (full rows, exact total) can only be cut from the WHOLE facet —
// which costs a walk, or a fan-out, that no single request should pay for.
//
// So the facet is built once and served from memory, and a stale build is
// served while the next one runs: a browse list may lag the catalog by minutes;
// it may not hang on a rebuild.

import (
	"context"
	"sync"
	"time"

	"kun-galgame-api/pkg/errors"
)

// staleCache holds the last successful build of one facet.
type staleCache[T any] struct {
	mu        sync.Mutex
	rows      []T
	ok        bool          // a build has succeeded at least once
	built     time.Time     // when that build finished
	buildingC chan struct{} // non-nil while a rebuild is in flight
}

// get returns the cached rows, rebuilding when they are missing or older than
// ttl. Only the very first caller (or one arriving after a failed build) waits
// for a build; everyone else is served the previous rows.
//
// A failed rebuild keeps the previous rows AND the previous timestamp, so the
// next caller retries rather than caching an outage for a whole ttl.
func (c *staleCache[T]) get(
	ctx context.Context,
	ttl time.Duration,
	build func(context.Context) ([]T, *errors.AppError),
) ([]T, *errors.AppError) {
	c.mu.Lock()
	rows, ok, fresh, building := c.rows, c.ok, time.Since(c.built) < ttl, c.buildingC
	if ok && fresh {
		c.mu.Unlock()
		return rows, nil
	}
	if building != nil {
		// Someone is already on it: serve what we have rather than queue a
		// second identical build behind the first.
		c.mu.Unlock()
		if ok {
			return rows, nil
		}
		<-building
		c.mu.Lock()
		rows, ok = c.rows, c.ok
		c.mu.Unlock()
		if !ok {
			return nil, errors.ErrInternal("获取列表失败")
		}
		return rows, nil
	}
	done := make(chan struct{})
	c.buildingC = done
	c.mu.Unlock()

	if ok {
		// Stale but usable: refresh out of band, on a context that outlives this
		// request — the caller's is cancelled the moment it responds.
		go func() {
			built, appErr := build(context.WithoutCancel(ctx))
			c.finish(built, appErr, done)
		}()
		return rows, nil
	}

	built, appErr := build(ctx)
	c.finish(built, appErr, done)
	if appErr != nil {
		return nil, appErr
	}
	return built, nil
}

// finish publishes a build's result and releases the in-flight marker.
func (c *staleCache[T]) finish(rows []T, appErr *errors.AppError, done chan struct{}) {
	c.mu.Lock()
	if appErr == nil {
		c.rows, c.ok, c.built = rows, true, time.Now()
	}
	c.buildingC = nil
	c.mu.Unlock()
	close(done)
}
