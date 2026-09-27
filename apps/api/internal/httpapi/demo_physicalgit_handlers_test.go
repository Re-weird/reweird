package httpapi

import (
	"net/http"
	"testing"
)

// TestDemoPhysicalGitActionsAreGatedToTheCanonicalDemoProject proves the
// simulated apply-restoration/apply-break actions cannot be used against a
// normal, real project -- there is no equivalent hardware-mutating action
// for real Physical Git projects.
func TestDemoPhysicalGitActionsAreGatedToTheCanonicalDemoProject(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)

	restoreResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/demo/apply-restoration", nil)
	if restoreResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("apply-restoration on a real project status = %d body=%s", restoreResponse.StatusCode, readBody(t, restoreResponse))
	}
	breakResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/demo/apply-break", nil)
	if breakResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("apply-break on a real project status = %d body=%s", breakResponse.StatusCode, readBody(t, breakResponse))
	}
}
