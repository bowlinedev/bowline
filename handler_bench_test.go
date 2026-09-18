package bowline

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
)

type benchInput struct {
	ID    int64    `json:"id" validate:"required"`
	Names []string `json:"names"`
}

type benchOutput struct {
	ID    int64    `json:"id"`
	Names []string `json:"names"`
	Total int32    `json:"total"`
}

func benchProc(ctx context.Context, in benchInput) (benchOutput, error) {
	return benchOutput{ID: in.ID, Names: in.Names, Total: int32(len(in.Names))}, nil
}

func rawHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /bench", func(w http.ResponseWriter, r *http.Request) {
		var in benchInput
		if err := json.Unmarshal([]byte(r.URL.Query().Get("input")), &in); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if in.ID == 0 {
			http.Error(w, "id required", 400)
			return
		}
		out, _ := benchProc(r.Context(), in)
		if out.Names == nil {
			out.Names = []string{}
		}
		body, _ := json.Marshal(out)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		w.Write(body)
	})
	return mux
}

var benchURL = "/bench?input=" + url.QueryEscape(`{"id":7,"names":["a","b","c"]}`)

func runBench(b *testing.B, h http.Handler) {
	req := httptest.NewRequest(http.MethodGet, benchURL, nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 200 {
			b.Fatalf("status %d", rec.Code)
		}
	}
}

func BenchmarkRawNetHTTP(b *testing.B) {
	runBench(b, rawHandler())
}

func BenchmarkBowline(b *testing.B) {
	runBench(b, NewRouter(Query("bench", benchProc)).Handler())
}

func TestOverheadBudget(t *testing.T) {
	if os.Getenv("BOWLINE_BENCH") == "" {
		t.Skip("set BOWLINE_BENCH=1 to run")
	}
	raw, bl := 0.0, 0.0
	for range 7 {
		raw = best(raw, testing.Benchmark(BenchmarkRawNetHTTP))
		bl = best(bl, testing.Benchmark(BenchmarkBowline))
	}
	ratio := bl / raw
	t.Logf("raw %.0f ns/op, bowline %.0f ns/op, ratio %.3f", raw, bl, ratio)
	if ratio > 1.05 {
		t.Fatalf("overhead %.1f%% exceeds the 5%% budget", (ratio-1)*100)
	}
}

func best(current float64, r testing.BenchmarkResult) float64 {
	ns := float64(r.NsPerOp())
	if current == 0 || ns < current {
		return ns
	}
	return current
}

var benchBody = []byte(`{"id":7,"names":["a","b","c"]}`)

func BenchmarkBowlinePost(b *testing.B) {
	h := NewRouter(Mutation("bench", benchProc)).Handler()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		req := httptest.NewRequest(http.MethodPost, "/bench", bytes.NewReader(benchBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 200 {
			b.Fatalf("status %d", rec.Code)
		}
	}
}

func BenchmarkBowlinePostLargeBody(b *testing.B) {
	names := make([]string, 2000)
	for i := range names {
		names[i] = "name-value-" + strconv.Itoa(i)
	}
	body, err := json.Marshal(benchInput{ID: 7, Names: names})
	if err != nil {
		b.Fatal(err)
	}
	h := NewRouter(Mutation("bench", benchProc)).Handler(MaxBodySize(8 << 20))
	b.ReportAllocs()
	b.SetBytes(int64(len(body)))
	b.ResetTimer()
	for range b.N {
		req := httptest.NewRequest(http.MethodPost, "/bench", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 200 {
			b.Fatalf("status %d", rec.Code)
		}
	}
}

type restInput struct {
	ID    int64  `json:"id"`
	Limit int32  `json:"limit"`
	Sort  string `json:"sort"`
}

func restProc(ctx context.Context, in restInput) (benchOutput, error) {
	return benchOutput{ID: in.ID, Names: []string{"ada"}, Total: in.Limit}, nil
}

func BenchmarkBowlineRESTPath(b *testing.B) {
	h := NewRouter(Query("get", restProc, Path("invoices/{id}"))).Handler()
	req := httptest.NewRequest(http.MethodGet, "/api/invoices/4821", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 200 {
			b.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
	}
}

func BenchmarkBowlineRESTQuery(b *testing.B) {
	h := NewRouter(Query("list", restProc, Path("invoices"))).Handler()
	req := httptest.NewRequest(http.MethodGet, "/api/invoices?limit=20&sort=created", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 200 {
			b.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
	}
}
