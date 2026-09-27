package domain

// CameraConfig is a project's optional vision camera source: an MJPEG
// (Motion JPEG over HTTP) stream, such as those exposed by common IP-camera
// and "IP Webcam" phone apps. It is stored on the Project itself (see
// Project.CameraConfig) rather than in a separate table, matching how the
// rest of Project's optional configuration (Repository, ProbePlan) is
// already modeled.
type CameraConfig struct {
	SourceType string `json:"source_type"`
	URL        string `json:"url"`
}

// CameraSourceMJPEG is the only supported CameraConfig.SourceType today.
const CameraSourceMJPEG = "mjpeg"
