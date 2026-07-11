// Per-course color assignment.
// Each course gets a stable, distinct color derived from its ID using the
// golden-angle hue spread (137.508°), so consecutive course IDs land on
// very different hues and the palette scales to any course count without
// collisions. Replaces the old department-based coloring.

/**
 * Returns inline CSS style for a course card: a pale tinted background plus a
 * solid left-border color. Stable for the same courseId across all views.
 */
export function courseColorStyle(courseId?: number): Record<string, string> {
  const id = courseId && courseId > 0 ? courseId : 0
  const hue = (((id * 137.508) % 360) + 360) % 360
  return {
    borderLeftColor: `hsl(${hue.toFixed(1)}, 58%, 42%)`,
    backgroundColor: `hsl(${hue.toFixed(1)}, 60%, 93%)`,
  }
}
