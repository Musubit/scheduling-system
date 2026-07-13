package types

// DayOfWeek is the internal 0-indexed weekday used across the scheduling
// pipeline. 0 = Monday, 6 = Sunday. Matches the semantics of
// backend/models.DayOfWeek without depending on it (INV-P2).
type DayOfWeek int

// Period is the internal 0-indexed teaching period. Period 0 = 第1节,
// Period 10 = 第11节. Matches the semantics of backend/models.Period
// without depending on it (INV-P2).
type Period int
