// Package types defines the DTOs and interfaces of the v0.5.5 two-stage
// scheduling pipeline (TimeScheduler → RoomScheduler → Orchestrator).
//
// This package depends only on the Go standard library (INV-P2). No GORM
// models, no database packages, no service-layer imports are allowed.
//
// The types here are plain value types (INV-P5, INV-H1). Interfaces are
// implementation-free; concrete implementations live in sibling packages
// (scheduling/time, scheduling/room, scheduling/orchestrator, scheduling/score).
//
// See docs/superpowers/specs/2026-07-13-scheduling-dual-mode-design.md
// (Sections 3.3, 3.4, and 7 Invariants) for the authoritative design.
package types
