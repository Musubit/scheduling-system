//go:build production

package database

// SeedData is a no-op in production builds.
// Users create their own data through the UI; no test fixtures are auto-seeded.
func SeedData(db DB) {}
