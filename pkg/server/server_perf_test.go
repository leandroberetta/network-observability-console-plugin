package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/netobserv/network-observability-console-plugin/pkg/config"
	"github.com/netobserv/network-observability-console-plugin/pkg/model"
)

const (
	benchmarkMaxIdleConns        = 100
	benchmarkMaxIdleConnsPerHost = 100
	benchmarkIdleConnTimeout     = 90 * time.Second
)

type filterTest struct {
	name   string
	filter string
}

// Helper function to setup mock servers for benchmarks with configurable result size
func setupBenchmarkServersWithSize(useBothDataSources bool, numRecords int) (*httptest.Server, *httptest.Server, *httptest.Server, *http.Client) {
	// Setup mock Loki service
	lokiMock := httpMock{}
	matrixResponse, err := json.Marshal(model.QueryResponse{
		Status: "",
		Data: model.QueryResponseData{
			ResultType: model.ResultTypeMatrix,
			Result:     model.Matrix{},
		},
	})
	if err != nil {
		panic("failed to marshal matrix benchmark response: " + err.Error())
	}

	// Generate mock stream data with specified number of records
	streams := model.Streams{}
	if numRecords > 0 {
		entries := make([]model.Entry, numRecords)
		// Use single timestamp for all entries to avoid calling time.Now() repeatedly
		timestamp := time.Now()
		logLine := `{"SrcAddr":"10.0.0.1","DstAddr":"10.0.0.2","SrcPort":8080,"DstPort":443,"Proto":6,"Bytes":1024,"Packets":10}`
		for i := 0; i < numRecords; i++ {
			entries[i] = model.Entry{
				Timestamp: timestamp,
				Line:      logLine,
			}
		}
		streams = append(streams, model.Stream{
			Labels:  map[string]string{"app": "test"},
			Entries: entries,
		})
	}
	streamResponse, err := json.Marshal(model.QueryResponse{
		Status: "",
		Data: model.QueryResponseData{
			ResultType: model.ResultTypeStream,
			Result:     streams,
		},
	})
	if err != nil {
		panic("failed to marshal stream benchmark response: " + err.Error())
	}

	lokiMock.On("ServeHTTP", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		req := args.Get(1).(*http.Request)
		w := args.Get(0).(http.ResponseWriter)

		// Return matrix for metric queries, stream for flow records
		if req.URL.Path == "/loki/api/v1/query_range" {
			query := req.URL.Query().Get("query")
			if len(query) > 0 && query[0] != '{' {
				_, _ = w.Write(matrixResponse)
				return
			}
		}
		_, _ = w.Write(streamResponse)
	})
	lokiSvc := httptest.NewServer(&lokiMock)

	cfg := &config.Config{
		Loki: config.Loki{
			URL:    lokiSvc.URL,
			Labels: []string{"SrcK8S_Namespace", "DstK8S_Namespace"},
		},
		Frontend: config.Frontend{},
	}

	var promSvc *httptest.Server
	// Setup Prometheus mock if needed for Auto mode
	if useBothDataSources {
		promMock := httpMock{}
		promResponse := []byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`)
		promMock.On("ServeHTTP", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			_, _ = args.Get(0).(http.ResponseWriter).Write(promResponse)
		})
		promSvc = httptest.NewServer(&promMock)
		cfg.Prometheus = config.Prometheus{URL: promSvc.URL}
	}

	// Setup auth mock
	authM := authMock{}
	authM.MockGranted()

	// Setup backend server
	backendRoutes := setupRoutes(context.TODO(), cfg, &authM)
	backendSvc := httptest.NewServer(backendRoutes)

	// Configure HTTP client with connection pooling to reduce port usage
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        benchmarkMaxIdleConns,
			MaxIdleConnsPerHost: benchmarkMaxIdleConnsPerHost,
			IdleConnTimeout:     benchmarkIdleConnTimeout,
		},
	}

	return lokiSvc, promSvc, backendSvc, client
}

// Helper function to setup mock servers for benchmarks (default 0 records)
func setupBenchmarkServers(useBothDataSources bool) (*httptest.Server, *httptest.Server, *httptest.Server, *http.Client) {
	return setupBenchmarkServersWithSize(useBothDataSources, 0)
}

// Helper to cleanup benchmark servers
func cleanupBenchmarkServers(lokiSvc, promSvc, backendSvc *httptest.Server) {
	lokiSvc.Close()
	if promSvc != nil {
		promSvc.Close()
	}
	backendSvc.Close()
}

// Common filter test data used across table, topology, and overview benchmarks
// Returns filter strings in URL-encoded format
func getCommonFilterTests() []filterTest {
	return []filterTest{
		{
			"SingleFilter",
			"SrcK8S_Namespace%3Ddefault",
		},
		{
			"TwoFilters",
			"SrcK8S_Namespace%3Ddefault%2CSrcPort%3D8080",
		},
		{
			"FourFilters",
			"SrcK8S_Namespace%3Ddefault%2CSrcPort%3D8080%2CDstK8S_Namespace%3Dkube-system%2CProto%3D6",
		},
	}
}

// Helper to run a single flow records query (used by table view benchmarks)
// Queries the /api/loki/flow/records endpoint
func runFlowRecordsQuery(b *testing.B, client *http.Client, url, query string) {
	req, err := http.NewRequest("GET", url+query, nil)
	if err != nil {
		b.Fatalf("Failed to create request for %s: %v", url+query, err)
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

// Helper to run a single metrics query (used by topology and overview benchmarks)
// Queries the /api/flow/metrics endpoint
func runMetricsQuery(b *testing.B, client *http.Client, url, query string) {
	req, err := http.NewRequest("GET", url+query, nil)
	if err != nil {
		b.Fatalf("Failed to create request for %s: %v", url+query, err)
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

// Helper to run multiple metrics queries (used by overview benchmarks)
func runMetricsQueries(b *testing.B, client *http.Client, url string, queries []string) {
	for _, query := range queries {
		runMetricsQuery(b, client, url, query)
	}
}

// BenchmarkExport measures Export Flows performance with CSV format across different scenarios
func BenchmarkExport(b *testing.B) {
	lokiSvc, promSvc, backendSvc, client := setupBenchmarkServers(false)
	defer cleanupBenchmarkServers(lokiSvc, promSvc, backendSvc)

	tests := []struct {
		name   string
		params string
	}{
		{
			"BasicCSV",
			"format=csv",
		},
		{
			"WithFilters",
			"format=csv&filters=SrcK8S_Namespace%3Ddefault",
		},
		{
			"WithLimit100",
			"format=csv&limit=100",
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				req, err := http.NewRequest("GET", backendSvc.URL+"/api/loki/export?"+tt.params, nil)
				if err != nil {
					b.Fatalf("Failed to create request for %s: %v", backendSvc.URL+"/api/loki/export?"+tt.params, err)
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
