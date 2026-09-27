package projectunderstanding

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/re-weird/reweird/apps/api/internal/codeanalysis"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/vision"
	componentcatalog "github.com/re-weird/reweird/packages/component-catalog"
)

type Service struct {
	code    codeanalysis.Analyzer
	vision  vision.Analyzer
	catalog []componentcatalog.Entry
}

func New(code codeanalysis.Analyzer, visionAnalyzer vision.Analyzer, catalog []componentcatalog.Entry) *Service {
	return &Service{code: code, vision: visionAnalyzer, catalog: catalog}
}

func (service *Service) Analyze(ctx context.Context, project domain.Project, imagePath string) (domain.ProjectAnalysis, domain.ProjectProfile) {
	analysis := domain.ProjectAnalysis{GeneratedAtMS: time.Now().UTC().UnixMilli()}
	if project.Code == nil {
		analysis.Code = domain.CodeAnalysis{Status: "NO_CODE", Parser: "deterministic-structure-v1", Pins: []domain.CodePinFinding{}, Includes: []string{}, Libraries: []string{}, Timing: []string{}, Warnings: []string{"No code was uploaded or pasted; the draft relies on user input and vision suggestions."}}
	} else {
		analysis.Code = service.code.Analyze(project.Code.Filename, project.Code.Text)
	}
	if project.Image == nil || imagePath == "" {
		analysis.Vision = domain.VisionAnalysis{Status: "NO_IMAGE", Components: []domain.VisionComponent{}, Relationships: []domain.VisionRelationship{}, Warnings: []string{"No project image was uploaded; vision analysis was not requested."}}
	} else {
		analysis.Vision = service.vision.Analyze(ctx, imagePath, project.Image.ContentType)
	}
	return analysis, service.merge(project, analysis)
}

// AnalyzeCommitImage runs only the Gemini Vision step against an
// already-resolved image -- no code analysis, no ProjectProfile merge. It
// is safe to call independently of project analysis and never mutates
// anything. It matches a detected component's CatalogID against the
// Component Catalog by name when Gemini did not already supply one; an
// unmatched name is left blank (uncertain), never forced.
func (service *Service) AnalyzeCommitImage(ctx context.Context, imagePath, contentType string) domain.VisionAnalysis {
	result := service.vision.Analyze(ctx, imagePath, contentType)
	for index, component := range result.Components {
		if component.CatalogID != "" {
			continue
		}
		if entry, ok := componentcatalog.Find(service.catalog, component.Name); ok {
			result.Components[index].CatalogID = entry.ID
		}
	}
	return result
}

func (service *Service) merge(project domain.Project, analysis domain.ProjectAnalysis) domain.ProjectProfile {
	now := time.Now().UTC().UnixMilli()
	profile := domain.ProjectProfile{
		ID: project.ID, ProjectID: project.ID, Version: 1, ProjectName: project.Name,
		Controller: project.Controller, LogicVoltage: project.LogicVoltage, Confirmed: false,
		Components: []domain.ComponentSpecification{}, Connections: []domain.ProfileConnection{}, Probes: []domain.ProbeConfiguration{},
		Conflicts: []domain.ProfileConflict{}, UnresolvedQuestions: []string{}, OperatingConditions: []string{fmt.Sprintf("Controller logic level: %.1f V", project.LogicVoltage)},
		AnalysisStatus: "DRAFT", CreatedAtMS: now, UpdatedAtMS: now,
	}
	if strings.TrimSpace(project.Description) != "" {
		profile.ExpectedBehavior = strings.TrimSpace(project.Description)
	} else {
		profile.ExpectedBehavior = "Expected project behavior requires user confirmation."
		profile.UnresolvedQuestions = append(profile.UnresolvedQuestions, "What should this project do during normal operation?")
	}

	componentIDs := inferComponents(analysis.Code.Pins, analysis.Code.Includes, analysis.Code.Libraries)
	for _, candidate := range analysis.Vision.Components {
		id := strings.TrimSpace(candidate.CatalogID)
		if id == "" {
			if entry, ok := componentcatalog.Find(service.catalog, candidate.Name); ok {
				id = entry.ID
			} else {
				id = slug(candidate.Name)
			}
		}
		componentIDs[id] = appendUniqueSource(componentIDs[id], domain.SourceVisionAI)
	}
	for id, sources := range componentIDs {
		entry, found := componentcatalog.Find(service.catalog, id)
		component := domain.ComponentSpecification{ID: id, Name: humanize(id), Sources: sources, Confidence: confidenceForSources(sources), Confirmed: false}
		if found {
			component.Name = entry.Name
			component.Source = sourceURL(entry)
			component.Sources = appendUniqueSource(component.Sources, domain.SourceCatalog)
			component.InterfaceType = entry.InterfaceType
			component.ExpectedBehavior = entry.ExpectedBehavior
			component.SafeMeasurementNotes = entry.SafeMeasurement
			if entry.OperatingVoltage != nil && entry.OperatingVoltage.Typical != nil {
				component.Properties = map[string]float64{"supply_voltage": *entry.OperatingVoltage.Typical}
			}
		}
		for _, candidate := range analysis.Vision.Components {
			if candidate.CatalogID == id || normalize(candidate.Name) == normalize(component.Name) {
				component.Confidence = max(component.Confidence, candidate.Confidence)
			}
		}
		profile.Components = append(profile.Components, component)
	}
	sort.Slice(profile.Components, func(i, j int) bool { return profile.Components[i].Name < profile.Components[j].Name })

	for _, pin := range analysis.Code.Pins {
		componentID := componentForPin(pin)
		componentName := componentID
		if entry, ok := componentcatalog.Find(service.catalog, componentID); ok {
			componentName = entry.Name
		}
		role := strings.ToUpper(pin.Symbol)
		connection := domain.ProfileConnection{
			ID: fmt.Sprintf("gpio-%d-%s", pin.GPIO, slug(pin.Symbol)), ComponentID: componentID, ComponentName: componentName,
			Role: role, GPIO: integer(pin.GPIO), Target: fmt.Sprintf("%s GPIO%d / %s %s", project.Controller, pin.GPIO, componentName, role),
			Direction: pin.Direction, Behavior: pin.Behavior, Expected: expectedFor(pin.Behavior), Confidence: pin.Confidence,
			Sources: []domain.ProjectFactSource{domain.SourceCodeStaticAnalysis}, Evidence: []domain.ProfileEvidence{{Value: fmt.Sprintf("GPIO%d", pin.GPIO), Source: domain.SourceCodeStaticAnalysis, Confidence: pin.Confidence}}, Required: true,
		}
		profile.Connections = append(profile.Connections, connection)
	}

	for _, relationship := range analysis.Vision.Relationships {
		role := strings.ToUpper(strings.TrimSpace(relationship.Role))
		index := findConnection(profile.Connections, role)
		if index >= 0 && relationship.GPIO != nil {
			connection := &profile.Connections[index]
			visionValue := fmt.Sprintf("GPIO%d", *relationship.GPIO)
			connection.Evidence = append(connection.Evidence, domain.ProfileEvidence{Value: visionValue, Source: domain.SourceVisionAI, Confidence: relationship.Confidence})
			if connection.GPIO != nil && *connection.GPIO != *relationship.GPIO {
				conflictID := "conflict-" + connection.ID
				profile.Conflicts = append(profile.Conflicts, domain.ProfileConflict{ID: conflictID, ConnectionID: connection.ID, Field: connection.ComponentName + " " + connection.Role, Options: []domain.ConflictOption{{Value: fmt.Sprintf("GPIO%d", *connection.GPIO), Source: domain.SourceCodeStaticAnalysis, Confidence: connection.Confidence}, {Value: visionValue, Source: domain.SourceVisionAI, Confidence: relationship.Confidence}}, RequiresConfirmation: true})
				profile.UnresolvedQuestions = append(profile.UnresolvedQuestions, "Resolve "+connection.ComponentName+" "+connection.Role+" GPIO disagreement.")
			} else {
				connection.Sources = appendUniqueSource(connection.Sources, domain.SourceVisionAI)
			}
		} else if relationship.GPIO != nil {
			componentName := firstNonEmpty(relationship.To, relationship.From, "Unknown component")
			profile.Connections = append(profile.Connections, domain.ProfileConnection{ID: fmt.Sprintf("vision-gpio-%d-%s", *relationship.GPIO, slug(role)), ComponentName: componentName, Role: role, GPIO: relationship.GPIO, Target: fmt.Sprintf("Possible %s GPIO%d / %s %s", project.Controller, *relationship.GPIO, componentName, role), Direction: "unknown", Behavior: behaviorFromRole(role), Expected: expectedFor(behaviorFromRole(role)), Confidence: relationship.Confidence, Sources: []domain.ProjectFactSource{domain.SourceVisionAI}, Evidence: []domain.ProfileEvidence{{Value: fmt.Sprintf("GPIO%d", *relationship.GPIO), Source: domain.SourceVisionAI, Confidence: relationship.Confidence}}, Required: false})
		}
	}
	if len(profile.Components) == 0 {
		profile.UnresolvedQuestions = append(profile.UnresolvedQuestions, "No component was identified. Add at least one component before confirmation.")
	}
	if len(profile.Connections) == 0 {
		profile.UnresolvedQuestions = append(profile.UnresolvedQuestions, "No measurable GPIO or signal connection was identified. Add a connection before confirmation.")
	}
	if analysis.Vision.Status == "VISION_SKIPPED" {
		profile.UnresolvedQuestions = append(profile.UnresolvedQuestions, "Vision was skipped; confirm components using the photo and code evidence.")
	}
	return profile
}

func inferComponents(pins []domain.CodePinFinding, includes, libraries []string) map[string][]domain.ProjectFactSource {
	result := make(map[string][]domain.ProjectFactSource)
	roles := make(map[string]bool)
	for _, pin := range pins {
		roles[strings.ToLower(pin.Symbol)] = true
		result[componentForPin(pin)] = appendUniqueSource(result[componentForPin(pin)], domain.SourceCodeStaticAnalysis)
	}
	joined := strings.ToLower(strings.Join(append(includes, libraries...), " "))
	if containsRole(roles, "trig") && containsRole(roles, "echo") {
		result["hc-sr04"] = appendUniqueSource(result["hc-sr04"], domain.SourceInferred)
	}
	if strings.Contains(joined, "servo") {
		result["sg90-servo"] = appendUniqueSource(result["sg90-servo"], domain.SourceInferred)
	}
	if strings.Contains(joined, "wire") {
		result["i2c-device"] = appendUniqueSource(result["i2c-device"], domain.SourceInferred)
	}
	return result
}

func componentForPin(pin domain.CodePinFinding) string {
	name := strings.ToLower(pin.Symbol)
	behavior := strings.ToLower(pin.Behavior)
	switch {
	case strings.Contains(name, "trig") || strings.Contains(name, "echo"):
		return "hc-sr04"
	case strings.Contains(name, "servo"):
		return "sg90-servo"
	case strings.Contains(name, "zmpt"):
		return "zmpt101b"
	case strings.Contains(name, "led"):
		return "led"
	case strings.Contains(name, "button") || strings.Contains(name, "switch"):
		return "push-button"
	case strings.Contains(behavior, "i2c") || strings.Contains(name, "sda") || strings.Contains(name, "scl"):
		return "i2c-device"
	case strings.Contains(name, "tx") || strings.Contains(name, "rx") || strings.Contains(name, "uart"):
		return "uart-device"
	case strings.Contains(behavior, "pwm"):
		return "pwm-output"
	case pin.Direction == "input":
		return "generic-digital-input"
	default:
		return "generic-digital-output"
	}
}

func expectedFor(behavior string) domain.ExpectedSignal {
	expected := domain.ExpectedSignal{SignalType: strings.ReplaceAll(behavior, "_", " "), Required: true, Stable: !strings.Contains(behavior, "pulse") && !strings.Contains(behavior, "pwm"), MaxDropouts: 0}
	if expected.SignalType == "unknown" || expected.SignalType == "" {
		expected.SignalType = "digital"
	}
	return expected
}

func findConnection(connections []domain.ProfileConnection, role string) int {
	role = normalize(role)
	for index, connection := range connections {
		if normalize(connection.Role) == role || strings.Contains(normalize(connection.Role), role) || strings.Contains(role, normalize(connection.Role)) {
			return index
		}
	}
	return -1
}

func behaviorFromRole(role string) string {
	lower := strings.ToLower(role)
	switch {
	case strings.Contains(lower, "echo"):
		return "pulse_input"
	case strings.Contains(lower, "trig"):
		return "digital_pulse"
	case strings.Contains(lower, "pwm") || strings.Contains(lower, "servo"):
		return "pwm_output"
	case strings.Contains(lower, "sda"):
		return "i2c_sda"
	case strings.Contains(lower, "scl"):
		return "i2c_scl"
	default:
		return "digital"
	}
}

func containsRole(roles map[string]bool, fragment string) bool {
	for role := range roles {
		if strings.Contains(role, fragment) {
			return true
		}
	}
	return false
}

func sourceURL(entry componentcatalog.Entry) string {
	if len(entry.Sources) == 0 {
		return ""
	}
	return entry.Sources[0].URL
}

func confidenceForSources(sources []domain.ProjectFactSource) float64 {
	for _, source := range sources {
		if source == domain.SourceCodeStaticAnalysis {
			return 1
		}
	}
	if len(sources) > 0 {
		return 0.7
	}
	return 0
}

func appendUniqueSource(values []domain.ProjectFactSource, value domain.ProjectFactSource) []domain.ProjectFactSource {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func normalize(value string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + 32
		}
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, value)
}
func slug(value string) string {
	value = strings.Trim(normalize(value), "-")
	if value == "" {
		return "unknown"
	}
	return value
}
func humanize(value string) string { return strings.Title(strings.ReplaceAll(value, "-", " ")) }
func integer(value int) *int       { return &value }
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
