package collect

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"srvmon/internal/config"
	"srvmon/internal/model"
)

func TestEndpointHTTPUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	got := CollectWatchedEndpoints([]config.EndpointConfig{{Name: "API", URL: srv.URL}}, 2*time.Second)
	if len(got) != 1 || !got[0].Up {
		t.Fatalf("expected up, got %+v", got)
	}
}

func TestEndpointHTTPWrongStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
	}))
	defer srv.Close()
	got := CollectWatchedEndpoints([]config.EndpointConfig{{Name: "API", URL: srv.URL, ExpectStatus: 200}}, 2*time.Second)
	if got[0].Up {
		t.Fatalf("503 should be down when expecting 200: %+v", got[0])
	}
}

func TestEndpointHTTPDown(t *testing.T) {
	got := CollectWatchedEndpoints([]config.EndpointConfig{{Name: "Yok", URL: "http://127.0.0.1:1/health"}}, 500*time.Millisecond)
	if got[0].Up {
		t.Fatalf("unreachable should be down: %+v", got[0])
	}
}

func TestEndpointTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := CollectWatchedEndpoints([]config.EndpointConfig{{Name: "TCP", TCP: ln.Addr().String()}}, time.Second)
	if !got[0].Up {
		t.Fatalf("open port should be up: %+v", got[0])
	}
	// closed port
	got = CollectWatchedEndpoints([]config.EndpointConfig{{Name: "TCP", TCP: "127.0.0.1:1"}}, 500*time.Millisecond)
	if got[0].Up {
		t.Fatalf("closed port should be down: %+v", got[0])
	}
}

func TestWatchFlattenCountsDown(t *testing.T) {
	s := model.Sample{Watch: []model.WatchStatus{
		{Name: "a", Up: true, State: "Çalışıyor"},
		{Name: "b", Up: false, State: "Durdu"},
		{Name: "c", Up: false, State: "Bilinmiyor"}, // undetermined: not counted
	}}
	v := Flatten(s)
	if v["watch.down"] != 1 {
		t.Fatalf("watch.down = %v want 1", v["watch.down"])
	}
}
