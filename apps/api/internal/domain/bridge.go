package domain

// BridgeBinding is server-owned pairing state. Tokens are stored only as hashes.
// Raw retains the unmodified firmware envelope; the explicit profile alias is
// applied only after device, sequence and wire-profile validation.
type BridgeBinding struct {
	LastError     string             `json:"last_error,omitempty"`
	ProjectID     string             `json:"project_id"`
	OwnerID       string             `json:"owner_id"`
	DeviceID      string             `json:"device_id"`
	WireProfileID string             `json:"wire_profile_id"`
	Revision      int                `json:"revision"`
	ProfileHash   string             `json:"profile_hash"`
	TokenHash     string             `json:"token_hash"`
	ShareHash     string             `json:"share_hash"`
	ExpiresAtMS   int64              `json:"expires_at_ms"`
	ReceivedAtMS  int64              `json:"received_at_ms"`
	SentAtMS      int64              `json:"sent_at_ms"`
	Raw           *TelemetryEnvelope `json:"raw,omitempty"`
}

type BridgeRepository interface {
	ActiveBridgeForDevice(deviceID string) (*BridgeBinding, error)
	SaveBridge(BridgeBinding) error
	GetBridge(projectID string) (*BridgeBinding, error)
}
