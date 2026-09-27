#pragma once
#if defined(REWEIRD_PATCH_HOST_TEST) && !defined(ESP_PLATFORM)
#include "../host-test/patch_provision_mock.h"
#else
// Hardware qualification record, reviewed with the dedicated protected stage.
// NEVER fill this from an API, environment flag, telemetry or inferred probe.
// The shipped board has no such stage: these values deliberately provision none.
namespace PatchProvision {
static constexpr bool Verified = false;
static constexpr int OutputPin = -1;
static constexpr int EnablePin = -1; // active HIGH; external pull-down mandatory
static constexpr int InterlockPin = -1; // physical jumper, external pull-down
static constexpr const char *QualificationID = "";
static constexpr const char *ProfileID = "";
static constexpr int ProfileRevision = 0;
static constexpr const char *ProbeMapHash = "";
static constexpr const char *TargetNode = ""; // one qualified isolated INPUT
}
#endif
