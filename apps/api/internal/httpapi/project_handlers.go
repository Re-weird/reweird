package httpapi

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/profiles"
	"github.com/re-weird/reweird/apps/api/internal/projects"
	"github.com/re-weird/reweird/apps/api/internal/projectunderstanding"
)

func (controller *Controller) createProject(ctx *fiber.Ctx) error {
	var input struct {
		Name         string  `json:"name"`
		Description  string  `json:"description"`
		Controller   string  `json:"controller"`
		LogicVoltage float64 `json:"logic_voltage"`
	}
	if err := ctx.BodyParser(&input); err != nil {
		return apiError(ctx, fiber.StatusBadRequest, "INVALID_JSON", "Project details must be valid JSON.")
	}
	id, err := projects.NewID(input.Name)
	if err != nil {
		return internalError(ctx, err)
	}
	project := domain.Project{ID: id, Name: strings.TrimSpace(input.Name), Description: strings.TrimSpace(input.Description), Controller: strings.TrimSpace(input.Controller), LogicVoltage: input.LogicVoltage, AnalysisStatus: domain.AnalysisPending}
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

func (controller *Controller) listProjects(ctx *fiber.Ctx) error {
	items, err := controller.repository.ListProjects()
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(items)
}

func (controller *Controller) getProject(ctx *fiber.Ctx) error {
	project, err := controller.findProject(ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	return ctx.JSON(project)
}

func (controller *Controller) uploadProjectImage(ctx *fiber.Ctx) error {
	project, err := controller.findProject(ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "Create the project before uploading an image.")
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
	media, err := projects.SaveImage(controller.uploadRoot, project.ID, header.Filename, file)
	if err != nil {
		return apiError(ctx, fiber.StatusUnsupportedMediaType, "INVALID_IMAGE", err.Error())
	}
	project.Image = media
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

func (controller *Controller) uploadProjectCode(ctx *fiber.Ctx) error {
	project, err := controller.findProject(ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "Create the project before uploading code.")
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
	if controller.understanding == nil {
		return apiError(ctx, fiber.StatusServiceUnavailable, "ANALYSIS_UNAVAILABLE", "Project understanding is not configured.")
	}
	project, err := controller.findProject(ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	if project.Image == nil && project.Code == nil {
		return apiError(ctx, fiber.StatusUnprocessableEntity, "EMPTY_PROJECT", "Upload an image or provide project code before analysis.")
	}
	project.AnalysisStatus = domain.AnalysisProcessing
	project.AnalysisError = ""
	if err := controller.repository.SaveProject(*project); err != nil {
		return internalError(ctx, err)
	}
	imagePath := ""
	if project.Image != nil {
		imagePath, err = projects.ResolveImage(controller.uploadRoot, project.Image.StorageRef)
		if err != nil {
			project.AnalysisStatus = domain.AnalysisFailed
			project.AnalysisError = "The stored image reference is invalid."
			_ = controller.repository.SaveProject(*project)
			return apiError(ctx, fiber.StatusUnprocessableEntity, "INVALID_IMAGE_REFERENCE", project.AnalysisError)
		}
	}
	analysis, profile := controller.understanding.Analyze(ctx.Context(), *project, imagePath)
	project.Analysis = &analysis
	project.AnalysisStatus = domain.AnalysisDraftReady
	project.ProbePlan = nil
	if err := profiles.Validate(profile); err != nil {
		project.AnalysisStatus = domain.AnalysisFailed
		project.AnalysisError = err.Error()
		_ = controller.repository.SaveProject(*project)
		return apiError(ctx, fiber.StatusUnprocessableEntity, "DRAFT_PROFILE_INVALID", err.Error())
	}
	if err := controller.repository.SaveProfile(profile); err != nil {
		return internalError(ctx, err)
	}
	if err := controller.repository.SaveProject(*project); err != nil {
		return internalError(ctx, err)
	}
	stored, err := controller.repository.GetProject(project.ID)
	if err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(fiber.Map{"project": stored, "analysis": analysis, "profile": profile})
}

func (controller *Controller) getProjectProfile(ctx *fiber.Ctx) error {
	if project, err := controller.findProject(ctx.Params("id")); err != nil {
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
	project, err := controller.findProject(ctx.Params("id"))
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
	submitted.CreatedAtMS = existing.CreatedAtMS
	submitted.UpdatedAtMS = time.Now().UTC().UnixMilli()
	submitted.Probes = nil
	markUserCorrections(&submitted, *existing)
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
	project, err := controller.findProject(ctx.Params("id"))
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
	if err := controller.repository.SaveProfile(*profile); err != nil {
		return internalError(ctx, err)
	}
	if err := controller.repository.SaveProject(*project); err != nil {
		return internalError(ctx, err)
	}
	return ctx.JSON(fiber.Map{"profile": profile, "probe_plan": plan})
}

func (controller *Controller) getProbePlan(ctx *fiber.Ctx) error {
	project, err := controller.findProject(ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	if project.ProbePlan == nil {
		return apiError(ctx, fiber.StatusConflict, "PROBE_PLAN_NOT_READY", "Confirm the Project Profile before requesting probe placement.")
	}
	return ctx.JSON(project.ProbePlan)
}

func (controller *Controller) confirmProbePlan(ctx *fiber.Ctx) error {
	project, err := controller.findProject(ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	if project.ProbePlan == nil {
		return apiError(ctx, fiber.StatusConflict, "PROBE_PLAN_NOT_READY", "Confirm the Project Profile before confirming probe connections.")
	}
	project.ProbePlan.Connected = true
	project.ProbePlan.ConnectedAtMS = time.Now().UTC().UnixMilli()
	if err := controller.repository.SaveProject(*project); err != nil {
		return internalError(ctx, err)
	}
	controller.mu.Lock()
	controller.profileID = project.ID
	controller.stage = domain.StageDiagnose
	controller.reference = nil
	controller.mu.Unlock()
	if scenario, ok := controller.source.(domain.ScenarioTelemetrySource); ok {
		scenario.SetStage(domain.StageDiagnose)
	}
	return ctx.JSON(project.ProbePlan)
}

func (controller *Controller) findProject(id string) (*domain.Project, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("project id is required")
	}
	return controller.repository.GetProject(id)
}

func resetProjectAnalysis(project *domain.Project) {
	project.Analysis = nil
	project.AnalysisStatus = domain.AnalysisPending
	project.AnalysisError = ""
	project.ProbePlan = nil
}

func markUserCorrections(profile *domain.ProjectProfile, existing domain.ProjectProfile) {
	componentSources := make(map[string][]domain.ProjectFactSource)
	for _, component := range existing.Components {
		componentSources[component.ID] = component.Sources
	}
	for index := range profile.Components {
		profile.Components[index].Sources = appendSource(componentSources[profile.Components[index].ID], domain.SourceUser)
		profile.Components[index].Confirmed = false
	}
	connectionSources := make(map[string][]domain.ProjectFactSource)
	connectionEvidence := make(map[string][]domain.ProfileEvidence)
	for _, connection := range existing.Connections {
		connectionSources[connection.ID] = connection.Sources
		connectionEvidence[connection.ID] = connection.Evidence
	}
	for index := range profile.Connections {
		profile.Connections[index].Sources = appendSource(connectionSources[profile.Connections[index].ID], domain.SourceUser)
		profile.Connections[index].Evidence = append(connectionEvidence[profile.Connections[index].ID], domain.ProfileEvidence{Value: userConnectionValue(profile.Connections[index]), Source: domain.SourceUser, Confidence: 1})
		profile.Connections[index].Confirmed = false
	}
	resolutionByID := make(map[string]string)
	for _, conflict := range profile.Conflicts {
		if conflict.Resolved {
			resolutionByID[conflict.ID] = conflict.Resolution
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
	questions := make([]string, 0, len(profile.UnresolvedQuestions))
	for _, question := range profile.UnresolvedQuestions {
		if strings.HasPrefix(question, "Resolve ") && allConflictsResolved(profile.Conflicts) {
			continue
		}
		questions = append(questions, question)
	}
	profile.UnresolvedQuestions = questions
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
