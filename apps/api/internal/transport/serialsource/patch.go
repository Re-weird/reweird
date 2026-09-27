package serialsource

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/re-weird/reweird/apps/api/internal/patchcontrol"
	"io"
	"time"
)

func (s *Source) PatchLive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return !s.closed && s.lastError == nil && s.latest != nil && time.Since(s.receivedAt) < 3*time.Second
}

func (s *Source) PatchExchange(ctx context.Context, c patchcontrol.Command) (patchcontrol.Reply, error) {
	s.patchMu.Lock()
	defer s.patchMu.Unlock()
	w, ok := s.port.(io.Writer)
	if !ok {
		return patchcontrol.Reply{}, errors.New("serial transport is read-only")
	}
	var id [16]byte
	if _, e := rand.Read(id[:]); e != nil {
		return patchcontrol.Reply{}, e
	}
	c.RequestID = hex.EncodeToString(id[:])
	c.Type = "patch_command"
	b, e := json.Marshal(c)
	if e != nil {
		return patchcontrol.Reply{}, e
	}
	b = append(b, '\n')
	// Serial library write timeout is not available. Never let a blocked write
	// postpone the caller's deadline; firmware's independent lease is authoritative.
	done := make(chan error, 1)
	go func() {
		n, err := w.Write(b)
		if err == nil && n != len(b) {
			err = io.ErrShortWrite
		}
		done <- err
	}()
	timeout := time.NewTimer(80 * time.Millisecond)
	defer timeout.Stop()
	select {
	case e = <-done:
		if e != nil {
			return patchcontrol.Reply{}, e
		}
	case <-ctx.Done():
		_ = s.port.Close()
		return patchcontrol.Reply{}, ctx.Err()
	case <-timeout.C:
		_ = s.port.Close()
		return patchcontrol.Reply{}, errors.New("PATCH serial write timeout; port closed")
	}
	for {
		select {
		case <-ctx.Done():
			return patchcontrol.Reply{}, ctx.Err()
		case <-timeout.C:
			return patchcontrol.Reply{}, errors.New("PATCH acknowledgement timeout")
		case r := <-s.patchReplies:
			if r.RequestID == c.RequestID {
				return r, nil
			}
		}
	}
}
func (s *Source) patchReply(line []byte) bool {
	var tag struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(line, &tag) != nil || tag.Type != "patch_status" {
		return false
	}
	var r patchcontrol.Reply
	if json.Unmarshal(line, &r) == nil && r.RequestID != "" {
		select {
		case s.patchReplies <- r:
		default:
		}
	}
	return true // Control replies never become measurements, even if malformed.
}
