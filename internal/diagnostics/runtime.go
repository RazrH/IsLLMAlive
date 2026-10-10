// Package diagnostics provides opt-in, local runtime snapshots.
package diagnostics

import (
	"crypto/fips140"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/metrics"
	"runtime/pprof"
	"time"
)

func RuntimeSummary() string {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	samples := []metrics.Sample{{Name: "/gc/heap/live:bytes"}}
	metrics.Read(samples)
	return fmt.Sprintf("Go: %s\nFIPS mode: %t\nGo crypto module: %s\n"+
		"Heap live bytes (last GC): %d\nHeap allocated bytes: %d\n"+
		"Heap in-use bytes: %d\nHeap idle bytes: %d\nHeap released bytes: %d\n"+
		"Stack in-use bytes: %d\nGC metadata bytes: %d\nOther runtime bytes: %d\n"+
		"Next GC heap bytes: %d\nGC cycles: %d\nRuntime retained bytes: %d\nGoroutines: %d\n",
		runtime.Version(), fips140.Enabled(), fips140.Version(), samples[0].Value.Uint64(), mem.HeapAlloc,
		mem.HeapInuse, mem.HeapIdle, mem.HeapReleased, mem.StackInuse, mem.GCSys, mem.OtherSys,
		mem.NextGC, mem.NumGC, mem.Sys-mem.HeapReleased, runtime.NumGoroutine())
}

// WriteProfiles writes only when explicitly requested. It neither starts a server
// nor forces GC. Profiles may contain sensitive data and should remain local.
func WriteProfiles(dir string) error {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	prefix := time.Now().UTC().Format("20060102T150405.000000000")
	for _, name := range []string{"heap", "allocs", "goroutine"} {
		f, err := os.OpenFile(filepath.Join(dir, prefix+"-"+name+".pprof"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		err = pprof.Lookup(name).WriteTo(f, 0)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
