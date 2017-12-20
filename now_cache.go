package throttle

import (
	"time"
	"sync"
)

var (
	mutex          sync.Mutex
	globalNowCache *nowCache
)

func cachedNow() time.Time {
	return globalNowCache.Now
}

const nowUpdateInterval = 100 * time.Millisecond

type nowCache struct {
	Now time.Time
}

func (c *nowCache) foreverTick() {
	for {
		time.Sleep(nowUpdateInterval)
		c.Now = time.Now()
	}
}

func maybeInitNowCache() {
	mutex.Lock()
	defer mutex.Unlock()

	if globalNowCache == nil {
		globalNowCache = &nowCache{Now: time.Now()}
		go globalNowCache.foreverTick()
	}
}
