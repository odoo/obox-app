package printer

import (
	"sync"
	"testing"

	"obox-app/internal/testutil"
)

func TestBuildSnapshot(t *testing.T) {
	// Keys in different order should yield identical snapshot
	keys1 := []string{"dev-B", "dev-A", "dev-C"}
	keys2 := []string{"dev-C", "dev-B", "dev-A"}

	snap1 := buildSnapshot(keys1)
	snap2 := buildSnapshot(keys2)

	testutil.ExpectedEqual(t, snap1, snap2)
	testutil.ExpectedEqual(t, snap1, "dev-A|dev-B|dev-C")

	// Empty keys
	testutil.ExpectedEqual(t, buildSnapshot([]string{}), "")
}

func TestPrinterCache_Lifecycle(t *testing.T) {
	cache := &printerCache{}

	keys := []string{"dev1", "dev2"}
	testutil.ExpectedTrue(t, cache.HasChanged(keys))

	available := []Info{
		{Id: "p1", Name: "Printer 1", Type: TypeReceipt},
	}
	unavailable := []UnavailableInfo{
		{Name: "Printer Bad", Error: "permission denied"},
	}

	cache.Update(keys, available, unavailable)

	// Now cache has been updated with keys -> HasChanged should be false
	testutil.ExpectedFalse(t, cache.HasChanged(keys))

	// HasUnavailable should be true
	testutil.ExpectedTrue(t, cache.HasUnavailable())

	// Get should return copy of available printers
	printers := cache.Get()
	testutil.ExpectedLen(t, printers, 1)
	testutil.ExpectedEqual(t, printers[0].Id, "p1")

	// Mutating returned slice should not mutate internal cache
	printers[0].Name = "MUTATED"
	testutil.ExpectedNotEqual(t, cache.Get()[0].Name, "MUTATED")

	// Update with new keys and no unavailable printers
	newKeys := []string{"dev1", "dev2", "dev3"}
	cache.Update(newKeys, available, nil)

	testutil.ExpectedFalse(t, cache.HasUnavailable())
	testutil.ExpectedFalse(t, cache.HasChanged(newKeys))
}

func TestPrinterCache_ConcurrentAccess(t *testing.T) {
	cache := &printerCache{}
	var wg sync.WaitGroup

	for i := range 20 {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			keys := []string{"dev1", "dev2"}
			if workerID%2 == 0 {
				cache.Update(keys, []Info{{Id: "p1"}}, nil)
			} else {
				_ = cache.HasChanged(keys)
				_ = cache.Get()
				_ = cache.HasUnavailable()
			}
		}(i)
	}

	wg.Wait()
}
