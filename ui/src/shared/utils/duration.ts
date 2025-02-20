import {TimeRange} from 'src/types'
import {Duration, DurationUnit} from 'src/types/ast'

export const parseDuration = (input: string): Duration[] => {
  const r = /([0-9]+)(y|mo|w|d|h|ms|s|m|us|µs|ns)/g
  const result = []

  let match = r.exec(input)

  if (!match) {
    throw new Error(`could not parse "${input}" as duration`)
  }

  while (match) {
    result.push({
      magnitude: +match[1],
      unit: match[2],
    })

    match = r.exec(input)
  }

  return result
}

const UNIT_TO_APPROX_MS = {
  ns: 1 / 1000000,
  µs: 1 / 1000,
  us: 1 / 1000,
  ms: 1,
  s: 1000,
  m: 1000 * 60,
  h: 1000 * 60 * 60,
  d: 1000 * 60 * 60 * 24,
  w: 1000 * 60 * 60 * 24 * 7,
  mo: 1000 * 60 * 60 * 24 * 30,
  y: 1000 * 60 * 60 * 24 * 365,
}

export const durationToMilliseconds = (duration: Duration[]): number =>
  duration.reduce(
    (sum, {magnitude, unit}) => sum + magnitude * UNIT_TO_APPROX_MS[unit],
    0
  )

/*
  Convert an amount of milliseconds to a duration string.

  The returned duration string will use the largest units possible, e.g.

      millisecondsToDuration(9_000_000)

  Will return `2h30m` rather than `9000000ms`.
*/
export const millisecondsToDuration = (value: number): string => {
  const unitsAndMs = Object.entries(UNIT_TO_APPROX_MS).sort(
    (a, b) => b[1] - a[1]
  ) as [DurationUnit, number][]

  const durations: Duration[] = []

  let unitIndex = 0
  let remainder = value

  while (unitIndex < unitsAndMs.length) {
    const [unit, unitAsMs] = unitsAndMs[unitIndex]
    const valueInUnit = remainder / unitAsMs

    durations.push({unit, magnitude: Math.floor(valueInUnit)})
    remainder = remainder - Math.floor(valueInUnit) * unitAsMs
    unitIndex += 1
  }

  return durations
    .filter(({magnitude}) => magnitude > 0)
    .reduce((s, {unit, magnitude}) => `${s}${magnitude}${unit}`, '')
}

export const areDurationsEqual = (a: string, b: string): boolean => {
  try {
    return (
      durationToMilliseconds(parseDuration(a)) ===
      durationToMilliseconds(parseDuration(b))
    )
  } catch {
    return false
  }
}

export const timeRangeToDuration = (timeRange: TimeRange): string => {
  if (timeRange.upper || !timeRange.lower || !timeRange.lower.includes('now')) {
    throw new Error('cannot convert time range to duration')
  }

  return timeRange.lower.replace(/\s/g, '').replace(/now\(\)-/, '')
}
