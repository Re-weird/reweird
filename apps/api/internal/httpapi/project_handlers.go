package httpapi

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/profiles"
	"github.com/re-weird/reweird/apps/api/internal/projects"
	"github.com/re-weird/reweird/apps/api/internal/projectunderstanding"
	"github.com/re-weird/reweird/apps/api/internal/telemetry"
)

func (controller *Controller) createProject(ctx *fiber.Ctx) error {
	var input struct {
		Name         string  `json:"name"`
		Description  string  `json:"description"`
		Controller   string  `json:"controller"`
		LogicVoltage float64 `json:"logic_voltage"`
		// Repository is an optional "owner/name" from the caller's GitHub
		// installation; pushes to its default branch drive code analysis.
		Repository string `json:"repository"`
	}
	if err := ctx.BodyParser(&input); err != nil {
		return apiError(ctx, fiber.StatusBadRequest, "INVALID_JSON", "Project details must be valid JSON.")
	}
	var linked *domain.LinkedRepository
	if fullName := strings.TrimSpace(input.Repository); fullName != "" {
		if strings.Count(fullName, "/") != 1 {
			return apiError(ctx, fiber.StatusUnprocessableEntity, "INVALID_REPOSITORY", "Repository must look like owner/name.")
		}
		var err error
		if linked, err = controller.linkRepository(ctx, fullName); linked == nil {
			if err != nil {
				return internalError(ctx, err)
			}
			return nil
		}
	}
	id, err := projects.NewID(input.Name)
	if err != nil {
		return internalError(ctx, err)
	}
	project := domain.Project{ID: id, OwnerID: ownerID(ctx), Name: strings.TrimSpace(input.Name), Description: strings.TrimSpace(input.Description), Controller: strings.TrimSpace(input.Controller), LogicVoltage: input.LogicVoltage, AnalysisStatus: domain.AnalysisPending, Visibility: domain.VisibilityPrivate, Repository: linked}
	if err := projects.Validate(project); err != nil {
		return apiError(ctx, fiber.StatusUnprocessableEntity, "INVALID_PROJECT", err.Error())
	}
	if err := controller.repository.SaveProject(project); err != nil {
		return internalError(ctx, err)
	}
	stored, err := controller.repository.GetProject(project.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.Status(fiber.StatusCreated).JSON(stored)
}

func (controller *Controller) updateProjectVisibility(ctx *fiber.Ctx) error {
	var input struct {
		Visibility domain.ProjectVisibility `json:"visibility"`
	}
	if err := ctx.BodyParser(&input); err != nil {
		return apiError(ctx, fiber.StatusBadRequest, "INVALID_JSON", "Visibility must be valid JSON.")
	}
	if input.Visibility != domain.VisibilityPrivate && input.Visibility != domain.VisibilityPublic {
		return apiError(ctx, fiber.StatusUnprocessableEntity, "INVALID_VISIBILITY", "Visibility must be \"private\" or \"public\".")
	}
	// Same lock as every other project-mutating handler: they read, modify, and
	// SaveProject the full payload, so without it one of them could write back
	// a visibility read before this change.
	controller.profileMu.Lock()
	defer controller.profileMu.Unlock()
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	if err := controller.repository.SetProjectVisibility(project.ID, input.Visibility); err != nil {
		return internalError(ctx, err)
	}
	stored, err := controller.repository.GetProject(project.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(stored)
}

func (controller *Controller) listProjects(ctx *fiber.Ctx) error {
	items, err := controller.repository.ListProjects()
	if err != nil {
		return internalError(ctx, err)
	}
	owner := ownerID(ctx)
	visible := make([]domain.Project, 0, len(items))
	for _, item := range items {
		if item.OwnerID == owner {
			visible = append(visible, item)
		}
	}
	return ctx.JSON(visible)
}

func (controller *Controller) getProject(ctx *fiber.Ctx) error {
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	return ctx.JSON(project)
}

func (controller *Controller) uploadProjectImage(ctx *fiber.Ctx) error {
	controller.profileMu.Lock()
	defer controller.profileMu.Unlock()
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "Create the project before uploading an image.")
	}
	confirmed, err := controller.projectProfileConfirmed(project.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	if confirmed {
		return apiError(ctx, fiber.StatusConflict, "PROFILE_CONFIRMED", "Confirmed profiles are immutable; create an explicit revision before replacing project inputs.")
	}
	header, err := ctx.FormFile("file")
	if err != nil {
		return apiError(ctx, fiber.StatusBadRequest, "IMAGE_REQUIRED", "Attach a PNG or JPEG in the multipart field named file.")
	}
	file, err := header.Open()
	if err != nil {
		return apiError(ctx, fiber.StatusBadRequest, "IMAGE_READ_FAILED", "The uploaded image could not be read.")
	}
	defer file.Close()
	if project.Image != nil {
		if err := projects.PruneObsoleteImages(controller.uploadRoot, project.ID, project.Image.StorageRef); err != nil {
			return internalError(ctx, err)
		}
	}
	codeBytes := int64(0)
	if project.Code != nil {
		codeBytes = project.Code.SizeBytes
	}
	media, err := projects.SaveImage(controller.uploadRoot, project.ID, header.Filename, codeBytes, file)
	if err != nil {
		if errors.Is(err, projects.ErrStorageLimit) || errors.Is(err, projects.ErrPayloadTooLarge) {
			return apiError(ctx, fiber.StatusRequestEntityTooLarge, "UPLOAD_LIMIT_EXCEEDED", err.Error())
		}
		return apiError(ctx, fiber.StatusUnsupportedMediaType, "INVALID_IMAGE", err.Error())
	}
	project.Image = media
	resetProjectAnalysis(project)
	if err := controller.repository.SaveProject(*project); err != nil {
		return internalError(ctx, err)
	}
	if err := projects.PruneObsoleteImages(controller.uploadRoot, project.ID, media.StorageRef); err != nil {
		return internalError(ctx, err)
	}
	stored, err := controller.repository.GetProject(project.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(stored)
}

func (controller *Controller) uploadProjectCode(ctx *fiber.Ctx) error {
	controller.profileMu.Lock()
	defer controller.profileMu.Unlock()
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "Create the project before uploading code.")
	}
	confirmed, err := controller.projectProfileConfirmed(project.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	if confirmed {
		return apiError(ctx, fiber.StatusConflict, "PROFILE_CONFIRMED", "Confirmed profiles are immutable; create an explicit revision before replacing project inputs.")
	}
	var code *domain.ProjectCode
	if strings.Contains(strings.ToLower(ctx.Get("Content-Type")), "multipart/form-data") {
		header, formErr := ctx.FormFile("file")
		if formErr != nil {
			return apiError(ctx, fiber.StatusBadRequest, "CODE_REQUIRED", "Attach a supported text source file in the multipart field named file.")
		}
		file, openErr := header.Open()
		if openErr != nil {
			return apiError(ctx, fiber.StatusBadRequest, "CODE_READ_FAILED", "The uploaded code could not be read.")
		}
		defer file.Close()
		code, err = projects.ReadCode(header.Filename, file)
	} else {
		var input struct {
			CodeText string `json:"code_text"`
			Filename string `json:"filename"`
		}
		if parseErr := ctx.BodyParser(&input); parseErr != nil {
			return apiError(ctx, fiber.StatusBadRequest, "INVALID_JSON", "Pasted code must be provided as JSON.")
		}
		if strings.TrimSpace(input.Filename) == "" {
			input.Filename = "pasted-code.ino"
		}
		code, err = projects.ReadCode(input.Filename, strings.NewReader(input.CodeText))
	}
	if err != nil {
		if errors.Is(err, projects.ErrPayloadTooLarge) {
			return apiError(ctx, fiber.StatusRequestEntityTooLarge, "UPLOAD_LIMIT_EXCEEDED", err.Error())
		}
		return apiError(ctx, fiber.StatusUnsupportedMediaType, "INVALID_CODE", err.Error())
	}
	project.Code = code
	resetProjectAnalysis(project)
	if err := controller.repository.SaveProject(*project); err != nil {
		return internalError(ctx, err)
	}
	stored, err := controller.repository.GetProject(project.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(stored)
}

func (controller *Controller) analyzeProject(ctx *fiber.Ctx) error {
	controller.profileMu.Lock()
	defer controller.profileMu.Unlock()
	if controller.understanding == nil {
		return apiError(ctx, fiber.StatusServiceUnavailable, "ANALYSIS_UNAVAILABLE", "Project understanding is not configured.")
	}
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	confirmed, err := controller.projectProfileConfirmed(project.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	if confirmed {
		return apiError(ctx, fiber.StatusConflict, "PROFILE_CONFIRMED", "Confirmed profiles are immutable; create an explicit revision before replacing project inputs.")
	}
	if project.Image == nil && project.Code == nil {
		return apiError(ctx, fiber.StatusUnprocessableEntity, "EMPTY_PROJECT", "Upload an image or provide project code before analysis.")
	}
	analysis, profile, failure, err := controller.analyzeLocked(ctx.Context(), project)
	if err != nil {
		return internalError(ctx, err)
	}
	if failure != nil {
		return apiError(ctx, fiber.StatusUnprocessableEntity, failure.code, failure.message)
	}
	stored, err := controller.repository.GetProject(project.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(fiber.Map{"project": stored, "analysis": analysis, "profile": profile})
}

// analysisFailure is an input problem that left the project FAILED, as
// opposed to a storage error.
type analysisFailure struct {
	code    string
	message string
}

// analyzeLocked turns the project's stored image and code into a draft
// Project Profile and saves both. The caller holds profileMu and has
// checked the profile isn't confirmed.
func (controller *Controller) analyzeLocked(ctx context.Context, project *domain.Project) (domain.ProjectAnalysis, domain.ProjectProfile, *analysisFailure, error) {
	project.AnalysisStatus = domain.AnalysisProcessing
	project.AnalysisError = ""
	if err := controller.repository.SaveProject(*project); err != nil {
		return domain.ProjectAnalysis{}, domain.ProjectProfile{}, nil, err
	}
	fail := func(code, message string) (domain.ProjectAnalysis, domain.ProjectProfile, *analysisFailure, error) {
		project.AnalysisStatus = domain.AnalysisFailed
		project.AnalysisError = message
		return domain.ProjectAnalysis{}, domain.ProjectProfile{}, &analysisFailure{code: code, message: message}, controller.repository.SaveProject(*project)
	}
	imagePath := ""
	if project.Image != nil {
		var err error
		imagePath, err = projects.ResolveImage(controller.uploadRoot, project.Image.StorageRef)
		if err != nil {
			return fail("INVALID_IMAGE_REFERENCE", "The stored image reference is invalid.")
		}
	}
	analysis, profile := controller.understanding.Analyze(ctx, *project, imagePath)
	if controller.source.Name() == "serial" {
		if frame, frameErr := controller.source.Latest(ctx); frameErr == nil && telemetry.Validate(frame) == nil {
			profile = projectunderstanding.SuggestPhysicalProbeMapping(profile, frame)
		}
	}
	project.Analysis = &analysis
	project.AnalysisStatus = domain.AnalysisDraftReady
	project.ProbePlan = nil
	if err := profiles.Validate(profile); err != nil {
		return fail("DRAFT_PROFILE_INVALID", err.Error())
	}
	if err := controller.repository.SaveProjectProfile(*project, profile); err != nil {
		return domain.ProjectAnalysis{}, domain.ProjectProfile{}, nil, err
	}
	return analysis, profile, nil, nil
}

func (controller *Controller) getProjectProfile(ctx *fiber.Ctx) error {
	if project, err := controller.findProject(ctx, ctx.Params("id")); err != nil {
		return internalError(ctx, err)
	} else if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	profile, err := controller.repository.GetProfile(ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if profile == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROFILE_NOT_FOUND", "Analyze the project to generate a draft profile.")
	}
	return ctx.JSON(profile)
}

func (controller *Controller) updateProjectProfile(ctx *fiber.Ctx) error {
	controller.profileMu.Lock()
	defer controller.profileMu.Unlock()
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	existing, err := controller.repository.GetProfile(project.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	if existing == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROFILE_NOT_FOUND", "Analyze the project before saving corrections.")
	}
	if existing.Confirmed {
		return apiError(ctx, fiber.StatusConflict, "PROFILE_CONFIRMED", "Confirmed profiles are immutable in this phase; create an explicit revision before changing one.")
	}
	var submitted domain.ProjectProfile
	if err := ctx.BodyParser(&submitted); err != nil {
		return apiError(ctx, fiber.StatusBadRequest, "INVALID_JSON", "Profile corrections must be valid JSON.")
	}
	submitted.ID = existing.ID
	submitted.ProjectID = existing.ProjectID
	submitted.Version = maxInt(existing.Version, 1)
	submitted.Confirmed = false
	submitted.ConfirmedAtMS = 0
	submitted.ConfirmedBy = ""
	submitted.AnalysisStatus = "DRAFT"
	submitted.CreatedAtMS = existing.CreatedAtMS
	submitted.UpdatedAtMS = existing.UpdatedAtMS
	submitted.Probes = nil
	markUserCorrections(&submitted, *existing)
	for index := range submitted.Components {
		submitted.Components[index].Confirmed = false
	}
	for index := range submitted.Connections {
		submitted.Connections[index].Confirmed = false
	}
	if profileDraftChanged(submitted, *existing) {
		submitted.Version++
	}
	submitted.UpdatedAtMS = time.Now().UTC().UnixMilli()
	if err := profiles.Validate(submitted); err != nil {
		return apiError(ctx, fiber.StatusUnprocessableEntity, "INVALID_PROFILE", err.Error())
	}
	if err := controller.repository.SaveProfile(submitted); err != nil {
		return internalError(ctx, err)
	}
	stored, err := controller.repository.GetProfile(submitted.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(stored)
}

func (controller *Controller) confirmProjectProfile(ctx *fiber.Ctx) error {
	controller.profileMu.Lock()
	defer controller.profileMu.Unlock()
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	profile, err := controller.repository.GetProfile(project.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	if profile == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROFILE_NOT_FOUND", "Analyze the project before confirming its profile.")
	}
	if profile.Confirmed {
		if project.ProbePlan == nil || project.ProbePlan.ProfileID != profile.ID {
			return apiError(ctx, fiber.StatusConflict, "PROBE_PLAN_NOT_READY", "Confirmed profile is missing its generated probe plan; manual review is required.")
		}
		return ctx.JSON(fiber.Map{"profile": profile, "probe_plan": project.ProbePlan})
	}
	if err := profiles.ValidateConfirmable(*profile); err != nil {
		return apiError(ctx, fiber.StatusConflict, "PROFILE_INCOMPLETE", err.Error())
	}
	plan, probeConfigurations, err := projectunderstanding.GenerateProbePlan(*profile)
	if err != nil {
		return apiError(ctx, fiber.StatusConflict, "NO_PROBE_MAPPING", err.Error())
	}
	now := time.Now().UTC().UnixMilli()
	profile.Confirmed = true
	profile.ConfirmedAtMS = now
	profile.ConfirmedBy = "user"
	profile.AnalysisStatus = "CONFIRMED"
	profile.Probes = probeConfigurations
	profile.UnresolvedQuestions = nil
	for index := range profile.Components {
		profile.Components[index].Confirmed = true
		profile.Components[index].Sources = appendSource(profile.Components[index].Sources, domain.SourceUser)
	}
	for index := range profile.Connections {
		profile.Connections[index].Confirmed = true
		profile.Connections[index].Sources = appendSource(profile.Connections[index].Sources, domain.SourceUser)
	}
	if err := profiles.Validate(*profile); err != nil {
		return apiError(ctx, fiber.StatusUnprocessableEntity, "INVALID_PROFILE", err.Error())
	}
	project.ProbePlan = &plan
	project.AnalysisStatus = domain.AnalysisConfirmed
	if err := controller.repository.SaveProjectProfile(*project, *profile); err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(fiber.Map{"profile": profile, "probe_plan": plan})
}

// reviseProjectProfile opens an explicit new revision of a confirmed
// profile. The confirmed revision is never edited in place: the draft gets
// version+1, the probe plan must be generated and confirmed again, and every
// Known Good learned under the previous revision becomes incompatible.
func (controller *Controller) reviseProjectProfile(ctx *fiber.Ctx) error {
	controller.profileMu.Lock()
	defer controller.profileMu.Unlock()
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	profile, err := controller.repository.GetProfile(project.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	if profile == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROFILE_NOT_FOUND", "Analyze the project before revising its profile.")
	}
	if !profile.Confirmed {
		return apiError(ctx, fiber.StatusConflict, "PROFILE_NOT_CONFIRMED", "This profile is still a draft; edit it directly.")
	}
	revision := *profile
	revision.Version = profile.Version + 1
	revision.Confirmed = false
	revision.ConfirmedAtMS = 0
	revision.ConfirmedBy = ""
	revision.AnalysisStatus = "DRAFT"
	revision.Probes = nil
	revision.UpdatedAtMS = time.Now().UTC().UnixMilli()
	revision.Components = append([]domain.ComponentSpecification(nil), profile.Components...)
	revision.Connections = append([]domain.ProfileConnection(nil), profile.Connections...)
	for index := range revision.Components {
		revision.Components[index].Confirmed = false
	}
	for index := range revision.Connections {
		revision.Connections[index].Confirmed = false
	}
	if err := profiles.Validate(revision); err != nil {
		return apiError(ctx, fiber.StatusUnprocessableEntity, "INVALID_PROFILE", err.Error())
	}
	project.ProbePlan = nil
	project.AnalysisStatus = domain.AnalysisDraftReady
	if err := controller.repository.SaveProjectProfile(*project, revision); err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(revision)
}

func (controller *Controller) getProbePlan(ctx *fiber.Ctx) error {
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	profile, err := controller.repository.GetProfile(project.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	if profile == nil || !profile.Confirmed || project.ProbePlan == nil || project.ProbePlan.ProfileID != profile.ID {
		return apiError(ctx, fiber.StatusConflict, "PROBE_PLAN_NOT_READY", "Confirm the Project Profile before requesting probe placement.")
	}
	return ctx.JSON(project.ProbePlan)
}

func (controller *Controller) confirmProbePlan(ctx *fiber.Ctx) error {
	controller.profileMu.Lock()
	defer controller.profileMu.Unlock()
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	profile, err := controller.repository.GetProfile(project.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	if profile == nil || !profile.Confirmed || project.ProbePlan == nil || project.ProbePlan.ProfileID != profile.ID {
		return apiError(ctx, fiber.StatusConflict, "PROBE_PLAN_NOT_READY", "Confirm the Project Profile before confirming probe connections.")
	}
	project.ProbePlan.Connected = true
	project.ProbePlan.ConnectedAtMS = time.Now().UTC().UnixMilli()
	if err := controller.repository.SaveProject(*project); err != nil {
		return internalError(ctx, err)
	}
	controller.mu.Lock()
	// The built-in simulator emits only the ultrasonic-demo frame shape. Keep it
	// isolated from real projects so demo samples can never masquerade as their
	// measurements. A serial source is activated after the user connects probes.
	if controller.source.Name() != "simulator" {
		controller.profileID = project.ID
	}
	controller.stage = domain.StageDiagnose
	controller.reference = nil
	controller.mu.Unlock()
	if scenario, ok := controller.source.(domain.ScenarioTelemetrySource); ok {
		scenario.SetStage(domain.StageDiagnose)
	}
	return ctx.JSON(project.ProbePlan)
}

// findProject fetches a project and enforces ownership: a project with an
// OwnerID is only ever returned to the matching verified caller. A mismatch
// returns (nil, nil) — the same "not found" response as a missing ID — so a
// non-owner (including an anonymous Demo Mode caller) can't distinguish
// "doesn't exist" from "exists but isn't yours."
func (controller *Controller) findProject(ctx *fiber.Ctx, id string) (*domain.Project, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("project id is required")
	}
	project, err := controller.repository.GetProject(id)
	if err != nil || project == nil {
		return project, err
	}
	if project.OwnerID != ownerID(ctx) {
		return nil, nil
	}
	return project, nil
}

func (controller *Controller) projectProfileConfirmed(projectID string) (bool, error) {
	profile, err := controller.repository.GetProfile(projectID)
	if err != nil {
		return false, err
	}
	return profile != nil && profile.Confirmed, nil
}

func resetProjectAnalysis(project *domain.Project) {
	project.Analysis = nil
	project.AnalysisStatus = domain.AnalysisPending
	project.AnalysisError = ""
	project.ProbePlan = nil
}

func markUserCorrections(profile *domain.ProjectProfile, existing domain.ProjectProfile) {
	resolutionByID := make(map[string]string)
	resolvedConnections := make(map[string]bool)
	for _, conflict := range profile.Conflicts {
		if conflict.Resolved {
			resolutionByID[conflict.ID] = conflict.Resolution
			resolvedConnections[conflict.ConnectionID] = true
		}
	}
	profile.Conflicts = existing.Conflicts
	for index := range profile.Conflicts {
		resolution := resolutionByID[profile.Conflicts[index].ID]
		if resolution == "" {
			continue
		}
		if !conflictHasOption(profile.Conflicts[index], resolution) && !strings.HasPrefix(strings.ToUpper(resolution), "GPIO") {
			continue
		}
		profile.Conflicts[index].Resolution = resolution
		profile.Conflicts[index].Resolved = true
		applyResolution(profile, profile.Conflicts[index])
	}

	existingComponents := make(map[string]domain.ComponentSpecification)
	for _, component := range existing.Components {
		existingComponents[component.ID] = component
	}
	for index := range profile.Components {
		current := &profile.Components[index]
		previous, found := existingComponents[current.ID]
		current.Confirmed = false
		if !found {
			current.Sources = []domain.ProjectFactSource{domain.SourceUser}
			continue
		}
		current.Sources = append([]domain.ProjectFactSource(nil), previous.Sources...)
		if componentFactsChanged(*current, previous) {
			current.Sources = appendSource(current.Sources, domain.SourceUser)
		}
	}

	existingConnections := make(map[string]domain.ProfileConnection)
	for _, connection := range existing.Connections {
		existingConnections[connection.ID] = connection
	}
	for index := range profile.Connections {
		current := &profile.Connections[index]
		previous, found := existingConnections[current.ID]
		current.Confirmed = false
		if !found {
			current.Sources = []domain.ProjectFactSource{domain.SourceUser}
			current.Evidence = []domain.ProfileEvidence{{Value: userConnectionValue(*current), Source: domain.SourceUser, Confidence: 1}}
			continue
		}
		current.Sources = append([]domain.ProjectFactSource(nil), previous.Sources...)
		current.Evidence = append([]domain.ProfileEvidence(nil), previous.Evidence...)
		if connectionFactsChanged(*current, previous) || resolvedConnections[current.ID] {
			current.Sources = appendSource(current.Sources, domain.SourceUser)
			current.Evidence = appendUserEvidence(current.Evidence, userConnectionValue(*current))
		}
	}
	questions := make([]string, 0, len(profile.UnresolvedQuestions))
	for _, question := range profile.UnresolvedQuestions {
		if strings.HasPrefix(question, "Resolve ") && allConflictsResolved(profile.Conflicts) {
			continue
		}
		questions = append(questions, question)
	}
	profile.UnresolvedQuestions = questions
}

func appendUserEvidence(values []domain.ProfileEvidence, value string) []domain.ProfileEvidence {
	for _, existing := range values {
		if existing.Source == domain.SourceUser && existing.Value == value {
			return values
		}
	}
	return append(values, domain.ProfileEvidence{Value: value, Source: domain.SourceUser, Confidence: 1})
}

func componentFactsChanged(current, previous domain.ComponentSpecification) bool {
	current.Sources, previous.Sources = nil, nil
	current.Confirmed, previous.Confirmed = false, false
	return !reflect.DeepEqual(current, previous)
}

func connectionFactsChanged(current, previous domain.ProfileConnection) bool {
	current.Sources, previous.Sources = nil, nil
	current.Evidence, previous.Evidence = nil, nil
	current.Confirmed, previous.Confirmed = false, false
	return !reflect.DeepEqual(current, previous)
}

func profileDraftChanged(current, previous domain.ProjectProfile) bool {
	current.Version = previous.Version
	current.CreatedAtMS = previous.CreatedAtMS
	current.UpdatedAtMS = previous.UpdatedAtMS
	current.Confirmed = previous.Confirmed
	current.ConfirmedAtMS = previous.ConfirmedAtMS
	current.ConfirmedBy = previous.ConfirmedBy
	current.Probes, previous.Probes = nil, nil
	if len(current.UnresolvedQuestions) == 0 {
		current.UnresolvedQuestions = nil
	}
	if len(previous.UnresolvedQuestions) == 0 {
		previous.UnresolvedQuestions = nil
	}
	return !reflect.DeepEqual(current, previous)
}

func applyResolution(profile *domain.ProjectProfile, conflict domain.ProfileConflict) {
	if conflict.ConnectionID == "" {
		return
	}
	for index := range profile.Connections {
		if profile.Connections[index].ID != conflict.ConnectionID {
			continue
		}
		value := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(conflict.Resolution)), "GPIO")
		gpio, err := strconv.Atoi(value)
		if err == nil && gpio >= 0 && gpio <= 99 {
			profile.Connections[index].GPIO = &gpio
			profile.Connections[index].Target = fmt.Sprintf("%s GPIO%d / %s %s", profile.Controller, gpio, profile.Connections[index].ComponentName, profile.Connections[index].Role)
		}
	}
}

func conflictHasOption(conflict domain.ProfileConflict, resolution string) bool {
	for _, option := range conflict.Options {
		if strings.EqualFold(option.Value, resolution) {
			return true
		}
	}
	return false
}

func allConflictsResolved(conflicts []domain.ProfileConflict) bool {
	for _, conflict := range conflicts {
		if conflict.RequiresConfirmation && !conflict.Resolved {
			return false
		}
	}
	return true
}

func appendSource(values []domain.ProjectFactSource, value domain.ProjectFactSource) []domain.ProjectFactSource {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func userConnectionValue(connection domain.ProfileConnection) string {
	if connection.GPIO != nil {
		return fmt.Sprintf("GPIO%d", *connection.GPIO)
	}
	return connection.Target
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func apiError(ctx *fiber.Ctx, status int, code, detail string) error {
	return ctx.Status(status).JSON(fiber.Map{"error": code, "detail": detail})
}
