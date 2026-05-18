package server

import (
	"net/http"
	"testing"
)

// BenchmarkTopologyAuto measures Topology View with Auto data source for all metric types
func BenchmarkTopologyAuto(b *testing.B) {
	lokiSvc, promSvc, backendSvc, client := setupBenchmarkServers(true)
	defer cleanupBenchmarkServers(lokiSvc, promSvc, backendSvc)

	tests := []struct {
		name  string
		query string
	}{
		{"Bytes", "/api/flow/metrics?dataSource=auto&aggregateBy=resource&function=rate&type=Bytes"},
		{"Packets", "/api/flow/metrics?dataSource=auto&aggregateBy=resource&function=rate&type=Packets"},
		{"DNSLatency", "/api/flow/metrics?dataSource=auto&aggregateBy=resource&function=avg&type=DnsLatencyMs"},
		{"RTT", "/api/flow/metrics?dataSource=auto&aggregateBy=resource&function=avg&type=TimeFlowRttNs"},
		{"Dropped", "/api/flow/metrics?dataSource=auto&aggregateBy=resource&function=rate&type=PktDropPackets"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				runMetricsQuery(b, client, backendSvc.URL, tt.query)
			}
		})
	}

	b.Run("BytesWithDrops", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			runMetricsQuery(b, client, backendSvc.URL,
				"/api/flow/metrics?dataSource=auto&aggregateBy=resource&function=rate&type=Bytes")
			runMetricsQuery(b, client, backendSvc.URL,
				"/api/flow/metrics?dataSource=auto&aggregateBy=resource&function=rate&type=PktDropPackets")
		}
	})
}

// BenchmarkFilterHeavyTopology measures topology view performance with complex filter combinations
func BenchmarkFilterHeavyTopology(b *testing.B) {
	lokiSvc, promSvc, backendSvc, client := setupBenchmarkServers(true)
	defer cleanupBenchmarkServers(lokiSvc, promSvc, backendSvc)

	filterTests := getCommonFilterTests()

	for _, tt := range filterTests {
		b.Run(tt.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				params := "dataSource=auto&aggregateBy=resource&function=rate&type=Bytes&filters=" + tt.filter
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

// BenchmarkConcurrentTopology measures Topology view performance under concurrent user load
func BenchmarkConcurrentTopology(b *testing.B) {
	lokiSvc, promSvc, backendSvc, client := setupBenchmarkServers(true)
	defer cleanupBenchmarkServers(lokiSvc, promSvc, backendSvc)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			url := backendSvc.URL + "/api/flow/metrics?dataSource=auto&aggregateBy=resource&function=rate&type=Bytes"
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				b.Fatalf("Failed to create request for %s: %v", url, err)
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

// BenchmarkTopologyAggregations measures Topology view with different aggregation levels
func BenchmarkTopologyAggregations(b *testing.B) {
	lokiSvc, promSvc, backendSvc, client := setupBenchmarkServers(true)
	defer cleanupBenchmarkServers(lokiSvc, promSvc, backendSvc)

	tests := []struct {
		name  string
		query string
	}{
		{"ByCluster", "/api/flow/metrics?dataSource=auto&aggregateBy=cluster&function=rate&type=Bytes"},
		{"ByZone", "/api/flow/metrics?dataSource=auto&aggregateBy=zone&function=rate&type=Bytes"},
		{"ByHost", "/api/flow/metrics?dataSource=auto&aggregateBy=host&function=rate&type=Bytes"},
		{"ByNamespace", "/api/flow/metrics?dataSource=auto&aggregateBy=namespace&function=rate&type=Bytes"},
		{"ByOwner", "/api/flow/metrics?dataSource=auto&aggregateBy=owner&function=rate&type=Bytes"},
		{"ByResource", "/api/flow/metrics?dataSource=auto&aggregateBy=resource&function=rate&type=Bytes"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				runMetricsQuery(b, client, backendSvc.URL, tt.query)
			}
		})
	}
}
