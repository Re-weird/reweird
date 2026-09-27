package cameracapture

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func validJPEG(t *testing.T) []byte {
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

func multipartMJPEGServer(t *testing.T, frames ...[]byte) *httptest.Server {
	t.Helper()
	boundary := "reweirdtestboundary"
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+boundary)
		w.WriteHeader(http.StatusOK)
		writer := multipart.NewWriter(w)
		_ = writer.SetBoundary(boundary)
		for _, frame := range frames {
			part, err := writer.CreatePart(map[string][]string{"Content-Type": {"image/jpeg"}})
			if err != nil {
				return
			}
			if _, err := part.Write(frame); err != nil {
				return
			}
		}
		_ = writer.Close()
	}))
}

func TestCaptureExtractsFrameFromValidMultipartStream(t *testing.T) {
	frame := validJPEG(t)
	server := multipartMJPEGServer(t, frame, frame)
	defer server.Close()

	result, err := Capture(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Capture() error = %v", err)
	}
	if result.Width != 4 || result.Height != 4 || result.ContentType != "image/jpeg" {
		t.Fatalf("result = %#v", result)
	}
}

func TestCaptureExtractsFrameFromDirectImageResponse(t *testing.T) {
	frame := validJPEG(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(frame)
	}))
	defer server.Close()

	result, err := Capture(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Capture() error = %v", err)
	}
	if result.Width != 4 || result.Height != 4 {
		t.Fatalf("result = %#v", result)
	}
}

func TestCaptureRejectsMalformedMultipartBoundary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("garbage"))
	}))
	defer server.Close()

	_, err := Capture(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected an error for a missing multipart boundary")
	}
}

func TestCaptureRejectsGarbagePartData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=xyz")
		w.WriteHeader(http.StatusOK)
		writer := multipart.NewWriter(w)
		_ = writer.SetBoundary("xyz")
		part, _ := writer.CreatePart(map[string][]string{"Content-Type": {"image/jpeg"}})
		_, _ = part.Write([]byte("not actually a jpeg"))
	}))
	defer server.Close()

	_, err := Capture(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected an error for a part that does not decode as an image")
	}
}

func TestCaptureRejectsHTMLResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body>camera control page</body></html>"))
	}))
	defer server.Close()

	result := Test(context.Background(), server.URL)
	if result.Status != StatusInvalidStream {
		t.Fatalf("status = %s, want INVALID_STREAM (result=%#v)", result.Status, result)
	}
}

func TestCaptureTimesOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.Header().Set("Content-Type", "image/jpeg")
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	result := Test(ctx, server.URL)
	if result.Status != StatusTimeout {
		t.Fatalf("status = %s, want TIMEOUT (result=%#v)", result.Status, result)
	}
}

func TestCaptureRejectsOversizedFrame(t *testing.T) {
	oversized := make([]byte, maxFrameBytes+1024)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(oversized)
	}))
	defer server.Close()

	_, err := Capture(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected an error for a frame exceeding the size limit")
	}
}

func TestCaptureRejectsConnectionFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := server.URL
	server.Close()

	result := Test(context.Background(), closedURL)
	if result.Status != StatusUnreachable {
		t.Fatalf("status = %s, want UNREACHABLE (result=%#v)", result.Status, result)
	}
}

func TestValidateURLRejectsMalformedURL(t *testing.T) {
	if _, err := ValidateURL("http://[::1"); err == nil {
		t.Fatal("expected an error for a malformed URL")
	}
	if _, err := ValidateURL(""); err == nil {
		t.Fatal("expected an error for an empty URL")
	}
}

func TestValidateURLRejectsUnsupportedScheme(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "ftp://example.com/frame.jpg", "javascript:alert(1)"} {
		if _, err := ValidateURL(raw); err == nil {
			t.Fatalf("expected %q to be rejected", raw)
		}
	}
}

func TestValidateURLRejectsCredentials(t *testing.T) {
	if _, err := ValidateURL("http://user:pass@10.0.0.5:4444"); err == nil {
		t.Fatal("expected embedded credentials to be rejected")
	}
}

func TestCaptureDoesNotFollowRedirects(t *testing.T) {
	// A camera that responds with a redirect (e.g. to a completely
	// different host/scheme it does not control, or one that was
	// compromised) must never have that redirect silently followed --
	// otherwise the redirect target would bypass ValidateURL entirely,
	// since only the original URL is ever checked.
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("the redirect target must never be contacted")
	}))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirector.Close()

	result := Test(context.Background(), redirector.URL)
	if result.Status != StatusInvalidStream {
		t.Fatalf("status = %s, want INVALID_STREAM for an unfollowed redirect (result=%#v)", result.Status, result)
	}
}

func TestValidateURLAcceptsPrivateLANAddress(t *testing.T) {
	// This is intentionally a LAN camera feature -- a private address like a
	// real Android phone on the same Wi-Fi must not be rejected.
	if _, err := ValidateURL("http://10.110.194.207:4444"); err != nil {
		t.Fatalf("expected a private LAN address to be accepted, got %v", err)
	}
}

func TestConnectedStatusRequiresADecodedFrame(t *testing.T) {
	frame := validJPEG(t)
	server := multipartMJPEGServer(t, frame)
	defer server.Close()

	result := Test(context.Background(), server.URL)
	if result.Status != StatusConnected || !result.FrameAvailable {
		t.Fatalf("result = %#v", result)
	}
}
