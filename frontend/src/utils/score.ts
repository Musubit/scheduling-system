/**
 * Score-related utility functions shared across components.
 */

/** Map a 0–100 score to a hex color: green ≥80, orange ≥60, red <60. */
export function scoreColor(score: number): string {
  if (score >= 80) return '#18a058'
  if (score >= 60) return '#f0a020'
  return '#d03050'
}

/** Star rating string (★/☆/½) from a raw score and its max value. */
export function starRating(score: number, max: number): string {
  if (max <= 0) return '☆☆☆☆☆'
  const stars = Math.max(0, Math.min(5, (score / max) * 5))
  const full = Math.floor(stars)
  const half = stars - full >= 0.25 ? 1 : 0
  const empty = 5 - full - half
  return '★'.repeat(full) + (half ? '½' : '') + '☆'.repeat(empty)
}
