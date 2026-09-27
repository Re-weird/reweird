package httpapi

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func testJPEGFrame(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 60), G: uint8(y * 60), B: 100, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, img, nil); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// testMJPEGServer serves one JPEG frame per request as a single-frame
// multipart/x-mixed-replace response, matching what a real MJPEG camera
// serves for one still capture.
func testMJPEGServer(t *testing.T, frame []byte) *httptest.Server {
	t.Helper()
	boundary := "reweirdtestboundary"
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+boundary)
		w.WriteHeader(http.StatusOK)
		writer := multipart.NewWriter(w)
		_ = writer.SetBoundary(boundary)
		part, err := writer.CreatePart(map[string][]string{"Content-Type": {"image/jpeg"}})
		if err != nil {
			return
		}
		_, _ = part.Write(frame)
		_ = writer.Close()
	}))
}

func TestSaveCameraConfigPersistsAndReturnsProject(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)

	response := doJSON(t, app, http.MethodPut, "/api/v1/projects/"+project.ID+"/camera-config", map[string]any{"source_type": "mjpeg", "url": "http://10.110.194.207:4444"})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("save status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	var saved domain.Project
	decodeBody(t, response, &saved)
	if saved.CameraConfig == nil || saved.CameraConfig.URL != "http://10.110.194.207:4444" || saved.CameraConfig.SourceType != "mjpeg" {
		t.Fatalf("camera config = %#v", saved.CameraConfig)
	}

	getResponse := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID, nil)
	var fetched domain.Project
	decodeBody(t, getResponse, &fetched)
	if fetched.CameraConfig == nil || fetched.CameraConfig.URL != "http://10.110.194.207:4444" {
		t.Fatalf("fetched camera config = %#v", fetched.CameraConfig)
	}
}

func TestSaveCameraConfigRejectsInvalidURL(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)

	for _, raw := range []string{"file:///etc/passwd", "not a url", ""} {
		response := doJSON(t, app, http.MethodPut, "/api/v1/projects/"+project.ID+"/camera-config", map[string]any{"url": raw})
		if response.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("url %q: status = %d body=%s", raw, response.StatusCode, readBody(t, response))
		}
	}
}

func TestSaveCameraConfigRejectsUnsupportedSourceType(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	response := doJSON(t, app, http.MethodPut, "/api/v1/projects/"+project.ID+"/camera-config", map[string]any{"source_type": "rtsp", "url": "http://10.0.0.5:4444"})
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s", response.StatusCode, readBody(t, response))
	}
}

func TestClearCameraConfigRemovesIt(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	doJSON(t, app, http.MethodPut, "/api/v1/projects/"+project.ID+"/camera-config", map[string]any{"url": "http://10.0.0.5:4444"})

	response := doJSON(t, app, http.MethodDelete, "/api/v1/projects/"+project.ID+"/camera-config", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("clear status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	var cleared domain.Project
	decodeBody(t, response, &cleared)
	if cleared.CameraConfig != nil {
		t.Fatalf("expected camera config to be cleared, got %#v", cleared.CameraConfig)
	}
}

func TestTestCameraConnectionReportsNotConfigured(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	response := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/camera/test", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	var result map[string]any
	decodeBody(t, response, &result)
	if result["status"] != "NOT_CONFIGURED" {
		t.Fatalf("result = %#v", result)
	}
}

func TestTestCameraConnectionSucceedsAgainstRealMJPEGServer(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	server := testMJPEGServer(t, testJPEGFrame(t))
	defer server.Close()

	response := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/camera/test", map[string]any{"url": server.URL})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	var result map[string]any
	decodeBody(t, response, &result)
	if result["status"] != "CONNECTED" || result["frame_available"] != true {
		t.Fatalf("result = %#v", result)
	}
}

func TestTestCameraConnectionReportsUnreachable(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := server.URL
	server.Close()

	response := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/camera/test", map[string]any{"url": closedURL})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	var result map[string]any
	decodeBody(t, response, &result)
	if result["status"] != "UNREACHABLE" {
		t.Fatalf("result = %#v", result)
	}
}

func TestTestCameraConnectionUsesSavedConfigWhenNoOverrideGiven(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	server := testMJPEGServer(t, testJPEGFrame(t))
	defer server.Close()
	doJSON(t, app, http.MethodPut, "/api/v1/projects/"+project.ID+"/camera-config", map[string]any{"url": server.URL})

	response := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/camera/test", nil)
	var result map[string]any
	decodeBody(t, response, &result)
	if result["status"] != "CONNECTED" {
		t.Fatalf("result = %#v", result)
	}
}

func TestCaptureTestFrameReturnsDecodedImageWithoutCreatingACommit(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	server := testMJPEGServer(t, testJPEGFrame(t))
	defer server.Close()

	response := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/camera/capture-test-frame", map[string]any{"url": server.URL})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	var result map[string]any
	decodeBody(t, response, &result)
	if result["width"].(float64) != 4 || result["height"].(float64) != 4 || result["image_base64"] == "" {
		t.Fatalf("result = %#v", result)
	}

	listResponse := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	var list struct {
		Count int `json:"count"`
	}
	decodeBody(t, listResponse, &list)
	if list.Count != 0 {
		t.Fatalf("capture-test-frame must never create a physical commit, count = %d", list.Count)
	}
}

func TestCaptureTestFrameFailsClearlyWhenCameraNotConfigured(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	response := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/camera/capture-test-frame", nil)
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s", response.StatusCode, readBody(t, response))
	}
}

func TestCameraConfigOwnershipIsolation(t *testing.T) {
	app := testOwnedApp(t)
	createResponse := doJSONAs(t, app, http.MethodPost, "/api/v1/projects", "owner-a", map[string]any{"name": "Alice's Rig", "controller": "ESP32", "logic_voltage": 3.3})
	var project domain.Project
	decodeBody(t, createResponse, &project)

	response := doJSONAs(t, app, http.MethodPut, "/api/v1/projects/"+project.ID+"/camera-config", "owner-b", map[string]any{"url": "http://10.0.0.5:4444"})
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("a different owner must not be able to configure another owner's camera: status = %d body=%s", response.StatusCode, readBody(t, response))
	}
}

func TestCameraConfigMalformedProjectID(t *testing.T) {
	app, _ := testApp(t)
	response := doJSON(t, app, http.MethodPost, "/api/v1/projects/does-not-exist-at-all/camera/test", nil)
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", response.StatusCode, readBody(t, response))
	}
}
