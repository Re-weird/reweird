// Package demodata seeds a deterministic, idempotent Physical Git demo
// project so a hackathon judge can see the real COMMIT -> DIFF -> RESTORE ->
// VERIFY workflow without any real ESP32, phone camera, GitHub connection,
// or live Gemini call. All evidence here is simulated, but every commit is
// created through the exact same domain/store code path a real project
// uses -- nothing is hardcoded in the frontend.
package demodata

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"time"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/projects"
)

// PhysicalGitDemoProjectID is a stable, well-known id (not randomly
// generated) so the demo is reachable at a fixed URL and so seeding can
// detect "does this already exist" without relying on display sequence
// numbers alone.
const PhysicalGitDemoProjectID = "physical-git-demo"

// ProviderSimulatedDemo marks a PhysicalCommitVisionAnalysis as seeded demo
// data rather than a real Gemini response. It reuses the existing
// PhysicalCommitVisionAnalysis.Provider string field (previously only ever
// set to "gemini") instead of adding a new model/table -- a real analysis
// and a demo analysis are never structurally different, only their
// provenance differs, and this makes that provenance impossible to miss.
const ProviderSimulatedDemo = "simulated_demo"

func demoCommitID(sequence int) string { return fmt.Sprintf("pcommit-%032x", sequence) }

// Seed creates the canonical three-commit Physical Git demo story if it does
// not already exist, and does nothing (successfully) if it does -- safe to
// call on every server start. It never calls Gemini and never touches
// GitHub. The project is anonymous (OwnerID == "") so any unauthenticated
// judge can reach it, exactly like the existing Demo Mode project pool.
func Seed(repository domain.Repository, uploadRoot string) error {
	existing, err := repository.GetProject(PhysicalGitDemoProjectID)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil // already seeded -- idempotent, no duplicate commits.
	}

	now := time.Now().UTC().UnixMilli()
	project := domain.Project{
		ID:             PhysicalGitDemoProjectID,
		OwnerID:        "",
		Name:           "Ultrasonic Robot Demo",
		Description:    "DEMO MODE - SIMULATED: a deterministic Physical Git walkthrough. Evidence is simulated; Physical Git behavior is real.",
		Controller:     "ESP32",
		LogicVoltage:   3.3,
		AnalysisStatus: domain.AnalysisConfirmed,
		Visibility:     domain.VisibilityPublic,
		CreatedAtMS:    now,
		UpdatedAtMS:    now,
	}
	if err := repository.SaveProject(project); err != nil {
		return err
	}

	workingProfile := buildProfile(false)
	if err := repository.SaveProfile(workingProfile); err != nil {
		return err
	}

	commitRepository, ok := repository.(domain.PhysicalCommitRepository)
	if !ok {
		return fmt.Errorf("demodata: repository does not implement PhysicalCommitRepository")
	}
	measurementRepository, ok := repository.(domain.MeasurementRepository)
	if !ok {
		return fmt.Errorf("demodata: repository does not implement MeasurementRepository")
	}
	visionRepository, ok := repository.(domain.PhysicalCommitVisionAnalysisRepository)
	if !ok {
		return fmt.Errorf("demodata: repository does not implement PhysicalCommitVisionAnalysisRepository")
	}

	// HW-001 -- working baseline.
	workingMeasurement, err := measurementRepository.SaveMeasurement(buildMeasurement(PhysicalGitDemoProjectID, echoWorking()))
	if err != nil {
		return err
	}
	hw1 := domain.PhysicalCommit{
		ID: demoCommitID(1), ProjectID: PhysicalGitDemoProjectID,
		Note:            "Working ultrasonic sensor baseline",
		ProfileSnapshot: profileCopy(workingProfile),
		MeasurementID:   &workingMeasurement.ID,
	}
	if err := attachDemoImage(uploadRoot, &hw1, color.RGBA{40, 180, 99, 255}); err != nil {
		return err
	}
	if _, err := commitRepository.SavePhysicalCommit(hw1); err != nil {
		return err
	}
	if err := seedVision(visionRepository, PhysicalGitDemoProjectID, hw1.ID, []domain.VisionComponent{
		{CatalogID: "generic-digital-output", Name: "ESP32", Confidence: 0.95, Source: domain.SourceVisionAI},
		{CatalogID: "hc-sr04", Name: "HC-SR04", Confidence: 0.93, Source: domain.SourceVisionAI},
	}); err != nil {
		return err
	}

	// HW-002 -- changed/broken state: ECHO moved to GPIO19, SG90 Servo
	// added, ECHO activity now missing.
	brokenProfile := buildProfile(true)
	if err := repository.SaveProfile(brokenProfile); err != nil {
		return err
	}
	brokenMeasurement, err := measurementRepository.SaveMeasurement(buildMeasurement(PhysicalGitDemoProjectID, echoBroken()))
	if err != nil {
		return err
	}
	hw2 := domain.PhysicalCommit{
		ID: demoCommitID(2), ProjectID: PhysicalGitDemoProjectID,
		Note:            "Sensor failure after hardware change",
		ProfileSnapshot: profileCopy(brokenProfile),
		MeasurementID:   &brokenMeasurement.ID,
	}
	if err := attachDemoImage(uploadRoot, &hw2, color.RGBA{196, 43, 43, 255}); err != nil {
		return err
	}
	if _, err := commitRepository.SavePhysicalCommit(hw2); err != nil {
		return err
	}
	if err := seedVision(visionRepository, PhysicalGitDemoProjectID, hw2.ID, []domain.VisionComponent{
		{CatalogID: "generic-digital-output", Name: "ESP32", Confidence: 0.95, Source: domain.SourceVisionAI},
		{CatalogID: "hc-sr04", Name: "HC-SR04", Confidence: 0.9, Source: domain.SourceVisionAI},
		{CatalogID: "sg90-servo", Name: "SG90 Servo", Confidence: 0.88, Source: domain.SourceVisionAI},
	}); err != nil {
		return err
	}

	// HW-003 -- restored: wiring back to GPIO18, ECHO activity present
	// again. Left as the project's CURRENT live profile, so a judge running
	// Verify against HW-001 right after seeding sees SUPPORTED immediately.
	restoredProfile := buildProfile(false)
	if err := repository.SaveProfile(restoredProfile); err != nil {
		return err
	}
	restoredMeasurement, err := measurementRepository.SaveMeasurement(buildMeasurement(PhysicalGitDemoProjectID, echoRestored()))
	if err != nil {
		return err
	}
	hw3 := domain.PhysicalCommit{
		ID: demoCommitID(3), ProjectID: PhysicalGitDemoProjectID,
		Note:            "Restored ultrasonic sensor wiring",
		ProfileSnapshot: profileCopy(restoredProfile),
		MeasurementID:   &restoredMeasurement.ID,
	}
	if err := attachDemoImage(uploadRoot, &hw3, color.RGBA{40, 180, 99, 255}); err != nil {
		return err
	}
	if _, err := commitRepository.SavePhysicalCommit(hw3); err != nil {
		return err
	}
	if err := seedVision(visionRepository, PhysicalGitDemoProjectID, hw3.ID, []domain.VisionComponent{
		{CatalogID: "generic-digital-output", Name: "ESP32", Confidence: 0.95, Source: domain.SourceVisionAI},
		{CatalogID: "hc-sr04", Name: "HC-SR04", Confidence: 0.94, Source: domain.SourceVisionAI},
	}); err != nil {
		return err
	}

	// The three commits above are the immutable canonical history. The
	// project's LIVE current state is deliberately left BROKEN (matching
	// HW-002) so a judge can watch Verify go from NOT_SUPPORTED to
	// SUPPORTED by calling ApplyRestoration -- if it were left already
	// "restored", there would be nothing left to demonstrate.
	return ApplyBreak(repository)
}

// ApplyRestoration overwrites ONLY the Physical Git demo project's live
// current ProjectProfile/measurement with the working fixture -- it never
// touches any other project, and it never mutates a Physical Commit. This
// is simulated-demo state only; it exists so a judge can watch the real
// Verify engine react to a genuine change without needing real hardware.
func ApplyRestoration(repository domain.Repository) error {
	return applyState(repository, false, echoRestored())
}

// ApplyBreak is the inverse of ApplyRestoration, used both at seed time and
// by a "Reset Demo" action so the interactive story can be replayed without
// restarting the server or touching the seeded commit history.
func ApplyBreak(repository domain.Repository) error {
	return applyState(repository, true, echoBroken())
}

func applyState(repository domain.Repository, broken bool, echo echoState) error {
	profile := buildProfile(broken)
	if err := repository.SaveProfile(profile); err != nil {
		return err
	}
	measurementRepository, ok := repository.(domain.MeasurementRepository)
	if !ok {
		return fmt.Errorf("demodata: repository does not implement MeasurementRepository")
	}
	_, err := measurementRepository.SaveMeasurement(buildMeasurement(PhysicalGitDemoProjectID, echo))
	return err
}

func seedVision(repository domain.PhysicalCommitVisionAnalysisRepository, projectID, commitID string, components []domain.VisionComponent) error {
	_, err := repository.SavePhysicalCommitVisionAnalysis(domain.PhysicalCommitVisionAnalysis{
		ID:               "pcvision-" + commitID[len("pcommit-"):],
		ProjectID:        projectID,
		PhysicalCommitID: commitID,
		Provider:         ProviderSimulatedDemo,
		Analysis: domain.VisionAnalysis{
			Status:        "VISION_COMPLETE",
			Components:    components,
			Relationships: []domain.VisionRelationship{},
			Warnings:      []string{"Simulated demo interpretation: no Gemini Vision call was made."},
		},
	})
	return err
}

func buildProfile(broken bool) domain.ProjectProfile {
	now := time.Now().UTC().UnixMilli()
	echoGPIO := 18
	components := []domain.ComponentSpecification{
		{ID: "esp32", Name: "ESP32", InterfaceType: "GPIO", Confirmed: true, Confidence: 1, Sources: []domain.ProjectFactSource{domain.SourceUser}},
		{ID: "hc-sr04", Name: "HC-SR04", InterfaceType: "digital pulse", Confirmed: true, Confidence: 1, Sources: []domain.ProjectFactSource{domain.SourceUser}},
	}
	if broken {
		echoGPIO = 19
		components = append(components, domain.ComponentSpecification{ID: "sg90-servo", Name: "SG90 Servo", InterfaceType: "PWM", Confirmed: true, Confidence: 1, Sources: []domain.ProjectFactSource{domain.SourceUser}})
	}
	trig := intPtr(5)
	echo := intPtr(echoGPIO)
	return domain.ProjectProfile{
		ID: PhysicalGitDemoProjectID, ProjectID: PhysicalGitDemoProjectID, Version: 1,
		ProjectName: "Ultrasonic Robot Demo", Controller: "ESP32", LogicVoltage: 3.3,
		Confirmed: true, ConfirmedAtMS: now, ConfirmedBy: "demo-seed",
		Components: components,
		Connections: []domain.ProfileConnection{
			{ID: "trig", ComponentID: "hc-sr04", ComponentName: "HC-SR04", Role: "TRIG", GPIO: trig, Target: fmt.Sprintf("ESP32 GPIO%d / HC-SR04 TRIG", *trig), Direction: "output", Behavior: "digital_pulse", Confirmed: true, Required: true, Sources: []domain.ProjectFactSource{domain.SourceUser}},
			{ID: "echo", ComponentID: "hc-sr04", ComponentName: "HC-SR04", Role: "ECHO", GPIO: echo, Target: fmt.Sprintf("ESP32 GPIO%d / HC-SR04 ECHO", *echo), Direction: "input", Behavior: "pulse_input", Confirmed: true, Required: true, Sources: []domain.ProjectFactSource{domain.SourceUser}},
		},
		Probes: []domain.ProbeConfiguration{
			{Probe: "TRIG", Role: "TRIG", Expected: domain.ExpectedSignal{SignalType: "digital pulse", Required: true, Stable: true}},
			{Probe: "ECHO", Role: "ECHO", Expected: domain.ExpectedSignal{SignalType: "pulse", Required: true, Stable: true}},
		},
		ExpectedBehavior: "The HC-SR04 emits a TRIG pulse and reports distance via an ECHO pulse width.",
		AnalysisStatus:   "DRAFT", CreatedAtMS: now, UpdatedAtMS: now,
	}
}

func profileCopy(profile domain.ProjectProfile) *domain.ProjectProfile {
	copied := profile
	return &copied
}

type echoState struct {
	present      bool
	pulseWidthUS float64
}

func echoWorking() echoState  { return echoState{present: true, pulseWidthUS: 1420} }
func echoBroken() echoState   { return echoState{present: false} }
func echoRestored() echoState { return echoState{present: true, pulseWidthUS: 1400} }

func buildMeasurement(profileID string, echo echoState) domain.MeasurementWindow {
	now := time.Now().UTC().UnixMilli()
	trigFacts := domain.DerivedFacts{Probe: "TRIG", Role: "TRIG", AverageVoltage: floatPtr(3.3), Stable: true, AveragePulseWidthUS: floatPtr(10)}
	echoFacts := domain.DerivedFacts{Probe: "ECHO", Role: "ECHO", AverageVoltage: floatPtr(3.3), Stable: echo.present, MissingExpectedActivity: !echo.present}
	if echo.present {
		echoFacts.AveragePulseWidthUS = floatPtr(echo.pulseWidthUS)
	}
	return domain.MeasurementWindow{
		ProfileID: profileID, Source: "simulator", DeviceID: "demo-esp32",
		CapturedAtMS: now, IngestedAtMS: now,
		Raw: domain.TelemetryEnvelope{ProfileID: profileID, DeviceID: "demo-esp32", CapturedAtMS: now},
		Analysis: domain.AnalysisResult{
			ProfileID: profileID, CapturedAtMS: now, WindowMS: 500,
			Probes: []domain.DerivedFacts{trigFacts, echoFacts},
		},
	}
}

// attachDemoImage generates a small, deterministic, solid-color JPEG (no
// external/copyrighted asset, no camera) and stores it through the exact
// same SavePhysicalCommitImage path a real upload uses, so it is a real
// stored Physical Commit image reference, not a decorative frontend asset.
func attachDemoImage(uploadRoot string, commit *domain.PhysicalCommit, fill color.RGBA) error {
	canvas := image.NewRGBA(image.Rect(0, 0, 320, 240))
	for y := 0; y < canvas.Bounds().Dy(); y++ {
		for x := 0; x < canvas.Bounds().Dx(); x++ {
			canvas.Set(x, y, fill)
		}
	}
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, canvas, &jpeg.Options{Quality: 80}); err != nil {
		return err
	}
	media, err := projects.SavePhysicalCommitImage(uploadRoot, commit.ProjectID, commit.ID, &buffer)
	if err != nil {
		return err
	}
	media.OriginalFilename = "demo-simulated.jpg"
	commit.Image = media
	return nil
}

func intPtr(value int) *int         { return &value }
func floatPtr(value float64) *float64 { return &value }
