package server

import (
	"testing"
)

// BenchmarkTable measures Table View performance with and without histogram
func BenchmarkTable(b *testing.B) {
	lokiSvc, promSvc, backendSvc, client := setupBenchmarkServers(true)
	defer cleanupBenchmarkServers(lokiSvc, promSvc, backendSvc)

	b.Run("Basic", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			runFlowRecordsQuery(b, client, backendSvc.URL, "/api/loki/flow/records")
		}
	})

	b.Run("WithHistogram", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Get flow records
			runFlowRecordsQuery(b, client, backendSvc.URL, "/api/loki/flow/records")

			// Get histogram data
			runMetricsQuery(b, client, backendSvc.URL,
				"/api/flow/metrics?dataSource=auto&function=count&aggregateBy=app&type=Flows")
		}
	})
}

// BenchmarkLargeResultSets measures performance with varying result set sizes
func BenchmarkLargeResultSets(b *testing.B) {
	sizes := []struct {
		name  string
		count int
	}{
		{"100records", 100},
		{"1000records", 1000},
		{"10000records", 10000},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			lokiSvc, promSvc, backendSvc, client := setupBenchmarkServersWithSize(false, size.count)
			defer cleanupBenchmarkServers(lokiSvc, promSvc, backendSvc)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				runFlowRecordsQuery(b, client, backendSvc.URL, "/api/loki/flow/records")
			}
		})
	}
}

// BenchmarkFilterHeavyTableView measures table view performance with complex filter combinations
func BenchmarkFilterHeavyTableView(b *testing.B) {
	lokiSvc, promSvc, backendSvc, client := setupBenchmarkServers(false)
	defer cleanupBenchmarkServers(lokiSvc, promSvc, backendSvc)

	filterTests := getCommonFilterTests()

	for _, tt := range filterTests {
		b.Run(tt.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				runFlowRecordsQuery(b, client, backendSvc.URL, "/api/loki/flow/records?filters="+tt.filter)
			}
		})
	}
}

// BenchmarkConcurrentTableView measures Table View performance under concurrent user load
func BenchmarkConcurrentTableView(b *testing.B) {
	lokiSvc, promSvc, backendSvc, client := setupBenchmarkServers(false)
	defer cleanupBenchmarkServers(lokiSvc, promSvc, backendSvc)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			runFlowRecordsQuery(b, client, backendSvc.URL, "/api/loki/flow/records")
		}
	})
}
