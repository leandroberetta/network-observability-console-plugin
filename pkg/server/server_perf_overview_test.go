package server

import (
	"net/http"
	"testing"
)

// Overview page query builders
func getBasicQueries(dataSource string) []string {
	return []string{
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=namespace&function=rate&type=Bytes",
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=app&function=rate&type=Bytes",
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=namespace&function=rate&type=Packets",
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=app&function=rate&type=Packets",
	}
}

func getDNSQueries(dataSource string) []string {
	return []string{
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=namespace&function=avg&type=DnsLatencyMs",
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=app&function=avg&type=DnsLatencyMs",
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=namespace&function=p90&type=DnsLatencyMs",
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=app&function=p90&type=DnsLatencyMs",
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=DnsName&function=count&type=DnsFlows",
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=DnsFlagsResponseCode&function=count&type=DnsFlows",
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=app&function=count&type=DnsFlows",
	}
}

func getRTTQueries(dataSource string) []string {
	return []string{
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=namespace&function=min&type=TimeFlowRttNs",
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=app&function=min&type=TimeFlowRttNs",
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=namespace&function=avg&type=TimeFlowRttNs",
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=app&function=avg&type=TimeFlowRttNs",
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=namespace&function=p90&type=TimeFlowRttNs",
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=app&function=p90&type=TimeFlowRttNs",
	}
}

func getDroppedQueries(dataSource string) []string {
	return []string{
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=namespace&function=rate&type=PktDropPackets",
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=app&function=rate&type=PktDropPackets",
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=namespace&function=rate&type=PktDropBytes",
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=app&function=rate&type=PktDropBytes",
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=PktDropLatestState&function=rate&type=PktDropPackets",
		"/api/flow/metrics?dataSource=" + dataSource + "&aggregateBy=PktDropLatestDropCause&function=rate&type=PktDropPackets",
	}
}

// BenchmarkOverviewAuto measures Overview Page with Auto data source for all scenarios
func BenchmarkOverviewAuto(b *testing.B) {
	lokiSvc, promSvc, backendSvc, client := setupBenchmarkServers(true)
	defer cleanupBenchmarkServers(lokiSvc, promSvc, backendSvc)

	b.Run("Basic", func(b *testing.B) {
		queries := getBasicQueries("auto")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			runMetricsQueries(b, client, backendSvc.URL, queries)
		}
	})

	b.Run("DNS", func(b *testing.B) {
		queries := append(getBasicQueries("auto"), getDNSQueries("auto")...)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			runMetricsQueries(b, client, backendSvc.URL, queries)
		}
	})

	b.Run("RTT", func(b *testing.B) {
		queries := append(getBasicQueries("auto"), getRTTQueries("auto")...)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			runMetricsQueries(b, client, backendSvc.URL, queries)
		}
	})

	b.Run("Dropped", func(b *testing.B) {
		queries := append(getBasicQueries("auto"), getDroppedQueries("auto")...)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			runMetricsQueries(b, client, backendSvc.URL, queries)
		}
	})

	b.Run("Full", func(b *testing.B) {
		queries := getBasicQueries("auto")
		queries = append(queries, getDNSQueries("auto")...)
		queries = append(queries, getRTTQueries("auto")...)
		queries = append(queries, getDroppedQueries("auto")...)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			runMetricsQueries(b, client, backendSvc.URL, queries)
		}
	})
}

// BenchmarkFilterHeavyOverview measures overview page performance with complex filter combinations
func BenchmarkFilterHeavyOverview(b *testing.B) {
	lokiSvc, promSvc, backendSvc, client := setupBenchmarkServers(true)
	defer cleanupBenchmarkServers(lokiSvc, promSvc, backendSvc)

	filterTests := getCommonFilterTests()

	for _, tt := range filterTests {
		b.Run(tt.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				params := "dataSource=auto&aggregateBy=namespace&function=rate&type=Bytes&filters=" + tt.filter
				req, err := http.NewRequest("GET", backendSvc.URL+"/api/flow/metrics?"+params, nil)
				if err != nil {
					b.Fatalf("Failed to create request for %s: %v", backendSvc.URL+"/api/flow/metrics?"+params, err)
				}
				resp, err := client.Do(req)
				if err != nil {
					b.Fatalf("Request failed: %v", err)
				}
				if resp.Body != nil {
					resp.Body.Close()
				}
				if resp.StatusCode != http.StatusOK {
					b.Fatalf("Expected 200, got %d", resp.StatusCode)
				}
			}
		})
	}
}

// BenchmarkConcurrentOverview measures Overview page performance under concurrent user load
func BenchmarkConcurrentOverview(b *testing.B) {
	lokiSvc, promSvc, backendSvc, client := setupBenchmarkServers(true)
	defer cleanupBenchmarkServers(lokiSvc, promSvc, backendSvc)

	queries := getBasicQueries("auto")

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			query := queries[i%len(queries)]
			i++
			req, err := http.NewRequest("GET", backendSvc.URL+query, nil)
			if err != nil {
				b.Fatalf("Failed to create request for %s: %v", backendSvc.URL+query, err)
			}
			resp, err := client.Do(req)
			if err != nil {
				b.Fatalf("Request failed: %v", err)
			}
			if resp.Body != nil {
				resp.Body.Close()
			}
			if resp.StatusCode != http.StatusOK {
				b.Fatalf("Expected 200, got %d", resp.StatusCode)
			}
		}
	})
}
