import test from "node:test";
import assert from "node:assert/strict";
import {scopeLiveRequest} from "./bridge-scope.mjs";

test("cloud live and tests use project-scoped owner routes",()=>{
  for(const path of ["/api/v1/session","/api/v1/telemetry/status","/api/v1/tests/current","/api/v1/tests/test-1/capture"])
    assert.equal(scopeLiveRequest(path,"/projects/cloud-bench"),`${path}?project_id=cloud-bench`);
});
test("practice, judge and other project APIs never inherit a cloud source",()=>{
  for(const page of ["/projects/demo","/projects/cloud-bench/simulator","/live/cloud-bench"])
    assert.equal(scopeLiveRequest("/api/v1/session",page),"/api/v1/session");
  assert.equal(scopeLiveRequest("/api/v1/simulator/scenario","/projects/cloud-bench"),"/api/v1/simulator/scenario");
  assert.equal(scopeLiveRequest("/api/v1/projects/other/profile","/projects/cloud-bench"),"/api/v1/projects/other/profile");
});
