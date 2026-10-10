package diagnostics

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestProfilesAreExplicitAndReadable(t *testing.T) {
	if err := WriteProfiles(""); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := WriteProfiles(dir); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("profiles=%d", len(entries))
	}
	for _, entry := range entries {
		f, err := os.Open(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		gz, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			t.Fatal(err)
		}
		gz.Close()
		f.Close()
	}
	if !strings.Contains(RuntimeSummary(), "Heap live bytes") {
		t.Fatal("missing heap measurement")
	}
}

func TestRuntimeSummaryMemoryBreakdown(t *testing.T) {
	fields := make(map[string]string)
	for line := range strings.SplitSeq(RuntimeSummary(), "\n") {
		if name, value, ok := strings.Cut(line, ": "); ok {
			fields[name] = value
		}
	}
	for _, name := range []string{"Heap allocated bytes", "Heap in-use bytes", "Heap idle bytes", "Heap released bytes", "Stack in-use bytes", "GC metadata bytes", "Other runtime bytes", "Next GC heap bytes", "GC cycles", "Runtime retained bytes"} {
		if _, err := strconv.ParseUint(fields[name], 10, 64); err != nil {
			t.Fatalf("%s must be a byte or count measurement: %q", name, fields[name])
		}
	}
	if _, err := strconv.ParseBool(fields["FIPS mode"]); err != nil {
		t.Fatalf("missing FIPS mode: %q", fields["FIPS mode"])
	}
	if fields["Go crypto module"] == "" {
		t.Fatal("missing crypto module version")
	}
}
