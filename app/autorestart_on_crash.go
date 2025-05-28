package app

import "time"

// TODO: rename to RateLimiter (+also rename BeginAttempt)
type RestartLimiter struct {
	recentRestarts []time.Time
}

const AutorestartLimit = 2
const AutorestartWindow = 60 * time.Second

func (rt *RestartLimiter) BeginAttempt() (attempt int, isAllowed bool) {
	rt.cleanupOld()
	if len(rt.recentRestarts) >= AutorestartLimit {
		return 0, false
	}
	rt.recentRestarts = append(rt.recentRestarts, time.Now())
	return len(rt.recentRestarts), true
}

func (rt *RestartLimiter) cleanupOld() {
	filtered := []time.Time{}
	for _, t := range rt.recentRestarts {
		if time.Since(t) > AutorestartWindow {
			continue
		}
		filtered = append(filtered, t)
	}
	rt.recentRestarts = filtered
}

func (rt *RestartLimiter) Clear() {
	rt.recentRestarts = nil
}
