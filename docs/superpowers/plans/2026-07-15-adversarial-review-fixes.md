# Adversarial Review Fixes — Plan

Base commit: 604475d (HEAD)
Goal: Fix all CRITICAL and HIGH bugs from adversarial review, plus key MEDIUM items.

## Global Constraints
- Single-user offline desktop app (Wails v3 + Go + Vue 3)
- First principles: minimal change, no over-engineering
- All existing tests must pass after each task

## Task 1: Fix `||` vs `??` for finalScore=0 (CRITICAL)
**Files:** `frontend/src/views/ReportPage.vue`
**Details:** Replace all `snap.finalScore || snap.totalScore` with `snap.finalScore ?? snap.totalScore` (4 locations). Use `??` instead of `||` so that `finalScore=0` is not treated as falsy.
**Verify:** Grep for `finalScore ||` should return 0 results after fix.

## Task 2: Fix totalWeight=0 division by zero (HIGH)
**Files:** `backend/services/scoring_service.go`
**Details:** After computing `totalWeight`, add guard:
```go
if totalWeight == 0 {
    totalWeight = enabledCount
}
```
Also fix `enabledCount == 0` case (already has `perCategoryMax = 25.0` fallback, OK).
**Verify:** `go test ./backend/services/ -run TestScore -v` passes.

## Task 3: Fix ORToolsClient shared http.Client.Timeout race (MEDIUM)
**Files:** `backend/services/ortools_client.go`
**Details:** In `Solve()`, create a per-request client copy instead of mutating shared field:
```go
reqClient := *c.client
reqClient.Timeout = timeout
resp, err := reqClient.Post(...)
```
**Verify:** `go build ./backend/...` compiles.

## Task 4: Fix zero weight treated as weight-1 (MEDIUM)
**Files:** `backend/services/scoring_service.go`
**Details:** In `getWeight`, when `weights` map is explicitly provided, treat missing/0 keys as 0 (disabled) instead of fallback to 1. Change logic:
```go
getWeight := func(key string) int {
    if weights != nil {
        if w, ok := weights[key]; ok {
            return w
        }
        return 0 // explicitly provided weights map, key not present = disabled
    }
    return 1 // no weights configured = equal weight
}
```
**Verify:** `go test ./backend/services/ -run TestCategoryMaxes -v` passes.

## Task 5: RestoreVersion save snapshot before restore (HIGH)
**Files:** `backend/services/version_service.go`
**Details:** Before hard-deleting current entries, create a backup version with source=ManualAdjust so user can undo. Call `CreateManualVersion` at start of `RestoreVersion`.
**Verify:** `go build ./backend/...` compiles.

## Task 6: RestoreVersion refresh schedule store (HIGH)
**Files:** `frontend/src/views/ScheduleCenterPage.vue`
**Details:** After successful restore, also call `scheduleStore.loadSchedule(appStore.currentSemesterId)` to refresh the live schedule entries.
**Verify:** No compile errors.

## Task 7: Regenerate Wails bindings (CRITICAL)
**Files:** `frontend/bindings/scheduling-system/backend/services/versionservice.ts`
**Details:** The `RestoreVersion` binding has a placeholder ID. User must run `wails generate module`. We can't run it here, but we can verify the backend compiles and note the requirement.
**Verify:** `go build ./backend/...` compiles. Note in commit message.
