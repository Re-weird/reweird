#pragma once
// Native test fixture only. ESP_PLATFORM always selects the real locked record.
namespace PatchProvision {
constexpr bool Verified=true;
constexpr int OutputPin=10,EnablePin=11,InterlockPin=12;
constexpr const char *QualificationID="TEST-NOT-HARDWARE";
constexpr const char *ProfileID="test-project";
constexpr int ProfileRevision=1;
constexpr const char *ProbeMapHash="test-map";
constexpr const char *TargetNode="isolated-input";
}
