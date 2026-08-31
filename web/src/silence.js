// A task in active is a claim that an agent is working on it. When the agent
// stops -- crashed, killed, out of context -- nothing retracts the claim, and
// the row looks exactly like one being worked on right now. The README already
// names this: "A task parked in active by an agent that stopped is a lie the
// board tells you every time you open it."
//
// The threshold was measured rather than guessed. Across 19 completed active
// stretches on a real instance the median was 2.1 minutes, the 90th percentile
// 4.2, and the longest legitimate stretch 35.5. Four hours is roughly seven
// times that longest one.
//
// It is deliberately far looser than the data requires, because the two errors
// are not symmetric. A stopped agent never writes again, so a late mark still
// catches it -- the case this exists for is opening the board in the morning
// after three agents ran overnight. A mark that fires on an agent still working
// teaches the human to ignore it, and then it is worse than nothing.
export const SILENT_AFTER_MS = 4 * 60 * 60 * 1000

// Silence is only meaningful in active. Every other status is either somebody's
// turn to act (blocked, review) or nobody's claim at all (backlog, queue, done),
// and none of them is asserting that work is happening right now.
export function isSilent(task) {
  return task.status === 'active' &&
    Date.now() - new Date(task.updated_at).getTime() > SILENT_AFTER_MS
}
