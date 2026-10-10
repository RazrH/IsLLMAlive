package providers

import (
	"fmt"
	"strings"
	"testing"

	"isllmalive/internal/config"
)

func BenchmarkDeepSeekParse(b *testing.B) {
	item := "<item><title>API degraded</title><link>incident</link><description>&lt;strong&gt;Status:&lt;/strong&gt; resolved&lt;strong&gt;Affected components:&lt;/strong&gt; API Service</description><pubDate>Thu, 28 May 2026 19:24:51 +0800</pubDate></item>"
	body := []byte("<rss><channel>" + strings.Repeat(item, 15) + "</channel></rss>")
	for _, count := range []int{5, 20} {
		b.Run(fmt.Sprintf("monitors-%d", count), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				for range count {
					result := parseDeepSeekRSS(body, config.MonitorConfig{Name: "DeepSeek", Components: []string{"API Service", "Web Chat Service"}}, deepSeekTestResult(), deepSeekTestNow())
					if result.Err != nil {
						b.Fatal(result.Err)
					}
				}
			}
		})
	}
}

func BenchmarkDeepSeekSharedSnapshot(b *testing.B) {
	item := "<item><title>API degraded</title><link>incident</link><description>&lt;strong&gt;Status:&lt;/strong&gt; resolved&lt;strong&gt;Affected components:&lt;/strong&gt; API Service</description><pubDate>Thu, 28 May 2026 19:24:51 +0800</pubDate></item>"
	body := []byte("<rss><channel>" + strings.Repeat(item, 15) + "</channel></rss>")
	for _, count := range []int{5, 20} {
		b.Run(fmt.Sprintf("monitors-%d", count), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				snapshot, err := decodeDeepSeekSnapshot(body, deepSeekTestNow())
				if err != nil {
					b.Fatal(err)
				}
				for range count {
					result := snapshot.evaluate(config.MonitorConfig{Name: "DeepSeek", Components: []string{"API Service", "Web Chat Service"}}, deepSeekTestResult())
					if result.Err != nil {
						b.Fatal(result.Err)
					}
				}
			}
		})
	}
}
