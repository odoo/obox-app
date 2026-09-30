package bluetooth

import "sync"

var defaultRFCOMMCache = newRFCOMMCache()

type rfcommCache struct {
	mu      sync.RWMutex
	entries map[string]int
}

func newRFCOMMCache() *rfcommCache {
	return &rfcommCache{
		entries: make(map[string]int),
	}
}

func (c *rfcommCache) getCachedRFCOMMChannel(address string) int {
	address, _ = ValidateAddress(address)
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.entries[address]
}

func (c *rfcommCache) setCachedRFCOMMChannel(address string, channel int) {
	if channel <= 0 {
		return
	}
	address, _ = ValidateAddress(address)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[address] = channel
}
