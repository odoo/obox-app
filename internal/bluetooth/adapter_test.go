package bluetooth

import (
	"sync"
	"testing"

	"epos-proxy/internal/testutil"
)

func TestNewRFCOMMCache(t *testing.T) {
	cache := newRFCOMMCache()
	testutil.ExpectedNotNil(t, cache)
	testutil.ExpectedNotNil(t, cache.entries)
}

func TestRFCOMMCache_Operations(t *testing.T) {
	cache := newRFCOMMCache()
	const mac = "11:22:33:44:55:66"

	// 1. Initial lookup (empty)
	ch := cache.getCachedRFCOMMChannel(mac)
	testutil.ExpectedEqual(t, ch, 0)

	// 2. Set channel
	cache.setCachedRFCOMMChannel(mac, 1)

	// 3. Retrieve channel
	ch = cache.getCachedRFCOMMChannel(mac)
	testutil.ExpectedEqual(t, ch, 1)

	// 4. Address normalization in cache lookup (lowercase / hyphen format)
	const macHyphen = "11-22-33-44-55-66"
	ch = cache.getCachedRFCOMMChannel(macHyphen)
	testutil.ExpectedEqual(t, ch, 1)

	const macLower = "11:22:33:44:55:66"
	ch = cache.getCachedRFCOMMChannel(macLower)
	testutil.ExpectedEqual(t, ch, 1)

	// 5. Update channel
	cache.setCachedRFCOMMChannel(mac, 3)
	ch = cache.getCachedRFCOMMChannel(mac)
	testutil.ExpectedEqual(t, ch, 3)

	// 6. Invalid channel ignored
	cache.setCachedRFCOMMChannel(mac, 0)
	ch = cache.getCachedRFCOMMChannel(mac)
	testutil.ExpectedEqual(t, ch, 3)
}

func TestRFCOMMCache_ConcurrentCacheAccess(t *testing.T) {
	cache := newRFCOMMCache()

	const mac = "12:34:56:78:9A:BC"
	var wg sync.WaitGroup

	for i := range 20 {
		wg.Add(2)
		ch := (i % 8) + 1

		// Concurrent writers
		go func(channel int) {
			defer wg.Done()
			cache.setCachedRFCOMMChannel(mac, channel)
		}(ch)

		// Concurrent readers
		go func() {
			defer wg.Done()
			_ = cache.getCachedRFCOMMChannel(mac)
		}()
	}

	wg.Wait()
}
