package httpapi

import (
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func TestTelemetryWebSocketPushesSessionOnConnectAndOnNewFrame(t *testing.T) {
	app, _ := testApp(t)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_ = app.Listener(listener)
	}()
	t.Cleanup(func() { _ = app.Shutdown() })

	url := "ws://" + listener.Addr().String() + "/api/v1/ws/telemetry"
	var connection *websocket.Conn
	for attempt := 0; attempt < 20; attempt++ {
		connection, _, err = websocket.DefaultDialer.Dial(url, nil)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer connection.Close()

	var first domain.Session
	if err := connection.ReadJSON(&first); err != nil {
		t.Fatalf("read first session: %v", err)
	}
	if first.RawTelemetry.ProfileID != "ultrasonic-demo" {
		t.Fatalf("first session = %#v", first)
	}

	scenarioResponse := doJSON(t, app, http.MethodPost, "/api/v1/simulator/scenario", map[string]any{"scenario_id": "timing-drift"})
	if scenarioResponse.StatusCode != http.StatusOK {
		t.Fatalf("scenario select status = %d body=%s", scenarioResponse.StatusCode, readBody(t, scenarioResponse))
	}

	if err := connection.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var second domain.Session
	if err := connection.ReadJSON(&second); err != nil {
		t.Fatalf("read pushed session after new frame: %v", err)
	}
	if second.RawTelemetry.Sequence == first.RawTelemetry.Sequence {
		t.Fatalf("expected a new sequence after selecting a scenario, got %d twice", second.RawTelemetry.Sequence)
	}
	if second.ScenarioID != "timing-drift" {
		t.Fatalf("pushed session = %#v, want scenario timing-drift", second)
	}
}
