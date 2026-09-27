// usb-bridge forwards validated Telemetry v2 from one USB device. It never
// writes commands to the serial port, buffers offline captures, or reads files.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/re-weird/reweird/apps/api/internal/transport/serialsource"
)

func main() {
	endpoint := flag.String("url", "", "HTTPS hosted ReWeird URL")
	project := flag.String("project", "", "cloud project ID")
	port := flag.String("port", "COM5", "USB serial port")
	flag.Parse()
	u, err := url.Parse(*endpoint)
	if err != nil || u.Host == "" || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		log.Fatal("--url must be an HTTPS origin without credentials, query or fragment")
	}
	if *project == "" || strings.ContainsAny(*project, "/\\?#") {
		log.Fatal("--project must be the cloud project ID")
	}
	token := strings.TrimSpace(os.Getenv("REWEIRD_BRIDGE_TOKEN"))
	if len(token) != 64 {
		log.Fatal("Set REWEIRD_BRIDGE_TOKEN to the pairing token shown by your project")
	}
	u.Path = "/api/v1/bridge/" + *project + "/frames"
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	source, err := serialsource.Open(*port, 115200)
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()
	go func() {
		if err := source.Run(ctx); err != nil {
			log.Printf("USB disconnected: %v. Reconnect and pair again after device reboot.", err)
			cancel()
		}
	}()
	client := &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	log.Printf("Read-only USB bridge started on %s; PATCH commands are not supported. Ctrl+C stops.", *port)
	timer := time.NewTicker(100 * time.Millisecond)
	defer timer.Stop()
	var last uint64
	var have bool
	var reported bool
	var lastUSBError string
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		e, err := source.Latest(ctx)
		usbError := source.Status().LastError
		if usbError != "" && usbError != lastUSBError {
			log.Printf("USB capture rejected: %s. After a reboot, stop, pair again and restart the bridge.", usbError)
			reported = false
		}
		lastUSBError = usbError
		if err != nil {
			continue
		}
		if have && e.Sequence <= last {
			continue
		}
		last = e.Sequence
		have = true
		// Use actual host receipt time, not upload time. Never retry old frames.
		sent := source.Status().ReceivedAt.UnixMilli()
		body, _ := json.Marshal(map[string]any{"sent_at_ms": sent, "frame": e})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
		if err != nil {
			log.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-ReWeird-Bridge", token)
		res, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Print("Cloud unavailable; capture dropped (no offline replay).")
			reported = false
			continue
		}
		payload, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		res.Body.Close()
		if res.StatusCode == 401 || res.StatusCode == 409 {
			log.Fatalf("Bridge requires attention (%d): %s", res.StatusCode, payload)
		}
		if res.StatusCode != 200 {
			log.Printf("Capture rejected (%d): %s", res.StatusCode, payload)
			reported = false
			continue
		}
		if !reported {
			fmt.Printf("LIVE: %s / %s → cloud project %s; REAL_SERIAL, PATCH LOCKED\n", e.DeviceID, e.ProfileID, *project)
			reported = true
		}
	}
}
