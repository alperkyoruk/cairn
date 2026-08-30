// One unit, always. Never "1d 4h", and never the word "ago" in a table column:
// the header says it once.
//
// Truncate, never round -- 59 minutes is 59m and 61 minutes is 1h. Weeks stop
// at four, because past that a relative figure stops carrying information and
// an absolute date carries more: a board whose bottom rows read "9 Aug" makes
// staleness visible rather than arithmetic.
//
// Seven branches and no date library, which is also a supply-chain answer.

const MINUTE = 60
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR
const WEEK = 7 * DAY

export function formatRelative(value, now = Date.now()) {
  if (!value) return ''
  const then = new Date(value)
  const seconds = Math.max(0, (now - then.getTime()) / 1000)

  if (seconds < MINUTE) return 'now'
  if (seconds < HOUR) return `${Math.floor(seconds / MINUTE)}m`
  if (seconds < DAY) return `${Math.floor(seconds / HOUR)}h`
  if (seconds < WEEK) return `${Math.floor(seconds / DAY)}d`
  if (seconds < 4 * WEEK) return `${Math.floor(seconds / WEEK)}w`

  const sameYear = then.getFullYear() === new Date(now).getFullYear()
  return then.toLocaleDateString('en-GB', {
    day: 'numeric',
    month: 'short',
    ...(sameYear ? {} : { year: '2-digit' }),
  })
}

// Worklog entries use absolute times: 20 Aug 09:12.
export function formatAbsolute(value) {
  if (!value) return ''
  return new Date(value).toLocaleString('en-GB', {
    day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit',
  }).replace(',', '')
}

// The full local timestamp, for the title attribute every relative time carries.
export function formatFull(value) {
  if (!value) return ''
  return new Date(value).toLocaleString()
}
