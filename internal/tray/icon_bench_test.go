package tray

import "testing"

func BenchmarkGenerateIcon(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := GenerateIcon("#69B72A"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCachedIcon(b *testing.B) {
	if _, err := cachedIcon("#69B72A"); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := cachedIcon("#69B72A"); err != nil {
			b.Fatal(err)
		}
	}
}
