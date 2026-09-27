// Package cameracapture connects to a project's configured MJPEG (Motion
// JPEG over HTTP) camera source and extracts exactly one still frame. It is
// a raw-evidence capture layer only: it never calls Gemini, never keeps a
// stream open longer than it takes to obtain the one frame it needs, and is
// only ever pointed at a URL the project owner configured -- never a
// caller-supplied arbitrary target outside that project's own camera config.
//
// This is deliberately a LAN camera feature: real hardware sits at private
// addresses (e.g. an Android phone's "IP Webcam" app on the same Wi-Fi as
// the backend), so this package does not block RFC1918/private addresses.
// It does still bound every request by scheme, timeout, and response size,
// and it only ever performs two narrow operations (test, capture) against
// the URL already scoped to one project -- it is not a generic URL fetcher.
package cameracapture

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// maxFrameBytes bounds a single captured/tested frame. Real camera
	// frames are a few hundred KB; this is generous but not unbounded.
	maxFrameBytes  = 8 * 1024 * 1024
	requestTimeout = 8 * time.Second
)

type Status string

const (
	StatusConnected     Status = "CONNECTED"
	StatusUnreachable   Status = "UNREACHABLE"
	StatusInvalidStream Status = "INVALID_STREAM"
	StatusTimeout       Status = "TIMEOUT"
	StatusNotConfigured Status = "NOT_CONFIGURED"
)

// Frame is one decoded still image extracted from the configured source.
type Frame struct {
	Bytes       []byte
	ContentType string
	Width       int
	Height      int
}

// TestResult is the outcome of a real connection attempt. Status is never
// CONNECTED unless a frame was actually decoded -- a syntactically valid URL
// is never enough on its own.
type TestResult struct {
	Status         Status
	ContentType    string
	FrameAvailable bool
	LatencyMS      int64
	Message        string
}

var (
	errTimeout       = errors.New("camera request timed out")
	errUnreachable   = errors.New("could not reach the configured camera")
	errInvalidStream = errors.New("camera did not return usable image/MJPEG data")
)

// ValidateURL rejects malformed URLs and anything other than http/https --
// no file://, no other scheme -- and rejects embedded credentials. It does
// not resolve or contact the host.
func ValidateURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("camera url is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("malformed camera url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("camera url must use http or https")
	}
	if parsed.Host == "" {
		return nil, errors.New("camera url must include a host")
	}
	if parsed.User != nil {
		return nil, errors.New("camera url must not contain credentials")
	}
	return parsed, nil
}

// Test performs a real connection attempt and reports what happened. It
// never sends anything to Gemini and never persists the frame it reads.
func Test(ctx context.Context, rawURL string) TestResult {
	started := time.Now()
	frame, err := Capture(ctx, rawURL)
	latencyMS := time.Since(started).Milliseconds()
	if err != nil {
		return classify(err, latencyMS)
	}
	return TestResult{Status: StatusConnected, ContentType: frame.ContentType, FrameAvailable: true, LatencyMS: latencyMS}
}

func classify(err error, latencyMS int64) TestResult {
	switch {
	case errors.Is(err, errTimeout):
		return TestResult{Status: StatusTimeout, LatencyMS: latencyMS, Message: err.Error()}
	case errors.Is(err, errInvalidStream):
		return TestResult{Status: StatusInvalidStream, LatencyMS: latencyMS, Message: err.Error()}
	default:
		return TestResult{Status: StatusUnreachable, LatencyMS: latencyMS, Message: err.Error()}
	}
}

// Capture connects to rawURL, extracts exactly one complete frame, validates
// that it decodes as a real image, and closes the connection -- it never
// keeps a stream open beyond what is needed for that one frame. It handles
// both a continuous "multipart/x-mixed-replace" MJPEG stream (extracting the
// first part) and a plain single-image response (image/jpeg or image/png),
// since some camera apps serve a still snapshot directly at the configured
// URL. Anything else (an HTML control page, a non-2xx status, an
// undecodable body) is reported as errInvalidStream, never silently
// accepted as a stream.
func Capture(ctx context.Context, rawURL string) (Frame, error) {
	parsed, err := ValidateURL(rawURL)
	if err != nil {
		return Frame{}, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return Frame{}, err
	}
	// A real MJPEG camera has no legitimate reason to redirect. Go's default
	// client follows redirects to ANY scheme/host, which would let a
	// malicious or compromised camera bounce this request past ValidateURL
	// (e.g. to a file:// URL or an unintended internal target) after the
	// initial URL already passed validation. Refusing to follow redirects
	// closes that off entirely -- a 3xx response is simply treated as an
	// invalid stream by the status check below.
	client := &http.Client{
		Timeout:       requestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	response, err := client.Do(request)
	if err != nil {
		if requestCtx.Err() != nil {
			return Frame{}, fmt.Errorf("%w: %v", errTimeout, err)
		}
		return Frame{}, fmt.Errorf("%w: %v", errUnreachable, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Frame{}, fmt.Errorf("%w: camera returned HTTP %d", errInvalidStream, response.StatusCode)
	}

	mediaType, params, parseErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if parseErr != nil {
		mediaType = strings.ToLower(strings.TrimSpace(strings.SplitN(response.Header.Get("Content-Type"), ";", 2)[0]))
	}

	var payload []byte
	switch {
	case strings.HasPrefix(mediaType, "multipart/"):
		boundary := params["boundary"]
		if boundary == "" {
			return Frame{}, fmt.Errorf("%w: multipart response is missing a boundary", errInvalidStream)
		}
		payload, err = readMultipartFrame(response.Body, boundary, maxFrameBytes)
	case strings.HasPrefix(mediaType, "image/"):
		payload, err = readBounded(response.Body, maxFrameBytes)
	default:
		return Frame{}, fmt.Errorf("%w: content type %q is not MJPEG or a still image", errInvalidStream, mediaType)
	}
	if err != nil {
		if requestCtx.Err() != nil {
			return Frame{}, fmt.Errorf("%w: %v", errTimeout, err)
		}
		return Frame{}, err
	}

	config, _, decodeErr := image.DecodeConfig(bytes.NewReader(payload))
	if decodeErr != nil {
		return Frame{}, fmt.Errorf("%w: frame did not decode as an image: %v", errInvalidStream, decodeErr)
	}
	return Frame{Bytes: payload, ContentType: http.DetectContentType(payload), Width: config.Width, Height: config.Height}, nil
}

// readMultipartFrame extracts exactly one frame from a
// "multipart/x-mixed-replace" MJPEG stream. It deliberately does not use
// Go's stdlib mime/multipart.Reader: that reader requires every boundary
// delimiter to be preceded by "\r\n" per RFC 2046, but real Android/IP
// camera apps observed in the field emit the next boundary immediately
// after the previous frame's last byte with no CRLF at all. Against such a
// stream, mime/multipart never recognizes the boundary, so it keeps reading
// past it into subsequent frames until the size limit trips -- exactly the
// failure this function exists to avoid.
//
// Instead, this reads the first part's header block itself and, when a
// Content-Length header is present (as these camera apps' frames do),
// reads exactly that many bytes -- independent of how the boundary after it
// is formatted. Only when Content-Length is absent does it fall back to
// scanning for the bare boundary bytes (accepting either a preceding CRLF
// or none) within the size limit.
func readMultipartFrame(body io.Reader, boundary string, max int64) ([]byte, error) {
	reader := bufio.NewReaderSize(body, 4096)
	delimiter := "--" + boundary

	first, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errInvalidStream, err)
	}
	if !strings.HasPrefix(strings.TrimRight(first, "\r\n"), delimiter) {
		return nil, fmt.Errorf("%w: multipart response did not start with its boundary", errInvalidStream)
	}

	contentLength := int64(-1)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errInvalidStream, err)
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			break
		}
		if key, value, ok := strings.Cut(trimmed, ":"); ok && strings.EqualFold(strings.TrimSpace(key), "content-length") {
			if parsed, parseErr := strconv.ParseInt(strings.TrimSpace(value), 10, 64); parseErr == nil {
				contentLength = parsed
			}
		}
	}

	if contentLength >= 0 {
		if contentLength == 0 {
			return nil, fmt.Errorf("%w: empty frame", errInvalidStream)
		}
		if contentLength > max {
			return nil, fmt.Errorf("%w: frame exceeds the %d byte limit", errInvalidStream, max)
		}
		payload := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return nil, fmt.Errorf("%w: %v", errInvalidStream, err)
		}
		return payload, nil
	}

	buffered, err := io.ReadAll(io.LimitReader(reader, max+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errInvalidStream, err)
	}
	if index := bytes.Index(buffered, []byte(delimiter)); index >= 0 {
		buffered = bytes.TrimRight(buffered[:index], "\r\n")
	} else if int64(len(buffered)) > max {
		return nil, fmt.Errorf("%w: frame exceeds the %d byte limit", errInvalidStream, max)
	}
	if len(buffered) == 0 {
		return nil, fmt.Errorf("%w: empty frame", errInvalidStream)
	}
	return buffered, nil
}

func readBounded(reader io.Reader, max int64) ([]byte, error) {
	payload, err := io.ReadAll(io.LimitReader(reader, max+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errInvalidStream, err)
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("%w: empty frame", errInvalidStream)
	}
	if int64(len(payload)) > max {
		return nil, fmt.Errorf("%w: frame exceeds the %d byte limit", errInvalidStream, max)
	}
	return payload, nil
}
