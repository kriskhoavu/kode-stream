package workspace

import (
	"strconv"
	"sync"
	"testing"
)

func TestScanLocksSerializeEachWorkspaceAndReleaseEntries(t *testing.T) {
	var locks scanLocks
	var wait sync.WaitGroup
	var counts [5]int
	for index := range 100 {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			key := index % len(counts)
			unlock := locks.lock(strconv.Itoa(key))
			counts[key]++
			unlock()
		}(index)
	}
	wait.Wait()
	for key, count := range counts {
		if count != 20 {
			t.Fatalf("workspace %d scan count=%d", key, count)
		}
	}
	locks.mu.Lock()
	remaining := len(locks.entries)
	locks.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("scan locks retained after scans: %d", remaining)
	}
}
