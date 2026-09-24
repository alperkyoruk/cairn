<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api.js'
import TaskCard from '../components/TaskCard.vue'
import { isSilent } from '../silence.js'

// Cairn's own statuses, in workflow order, rather than renamed into somebody
// else's vocabulary. Left to right is the path work takes, which is the thing
// the state machine already describes; the columns the human has to act on are
// louder instead of being first, because reordering them would make the board
// disagree with the workflow it is drawn from.
//
// done is absent for the same reason it is absent from every listing: a
// finished task is rarely what anyone is looking for. It stays reachable on
// the project page.
const COLUMNS = [
  { status: 'backlog', label: 'Backlog', hint: 'not yours to start until you queue it' },
  { status: 'queue', label: 'Queue', hint: 'any agent may pick these up' },
  { status: 'active', label: 'Active', hint: 'claimed, work in progress' },
  { status: 'blocked', label: 'Blocked', hint: 'stopped, waiting on you' },
  { status: 'review', label: 'Review', hint: 'finished, waiting on your decision' },
]

// A bulk move carries no note, so it can only offer moves that do not ask for
// one. Two do: anything into blocked needs blocked_on, and review -> active is
// a rejection, which owes the agent a reason. Both are per-task judgements
// anyway -- one blocker shared across nine tasks is almost never true, and a
// rejection reason never is.
//
// This narrows what is offered; it does not decide anything. The server is
// still the only authority and refuses whatever it should, per task.
const NEEDS_A_NOTE = (from, to) => to === 'blocked' || (from === 'review' && to === 'active')

const MOVE_LABEL = {
  queue: 'Queue',
  active: 'Start',
  review: 'Send to review',
  done: 'Mark done',
  backlog: 'Move to backlog',
}

const router = useRouter()

const rows = ref([])
const agents = ref([])
const projects = ref([])
const failure = ref('')
const ready = ref(false)
const busy = ref(false)
const report = ref('')

const selected = ref(new Set())
const lastPicked = ref(null)

async function load() {
  try {
    const [board, agentList, projectList] = await Promise.all([
      api.board(), api.agents(), api.projects(),
    ])
    rows.value = board
    agents.value = agentList
    projects.value = projectList
    // Drop anything that has moved out from under the selection, so a stale
    // ref cannot ride along into the next bulk move.
    const live = new Set(board.map((r) => r.task.ref))
    selected.value = new Set([...selected.value].filter((ref) => live.has(ref)))
  } catch (err) {
    failure.value = err.message
  } finally {
    ready.value = true
  }
}

const columns = computed(() =>
  COLUMNS.map((col) => ({
    ...col,
    rows: rows.value.filter((r) => r.task.status === col.status),
  })))

const silentCount = computed(() => rows.value.filter((r) => isSilent(r.task)).length)
const selectedRows = computed(() => rows.value.filter((r) => selected.value.has(r.task.ref)))

// What may be done to every task in the selection, which is the intersection of
// what may be done to each -- offering a move that is legal for eight of nine
// would refuse on the ninth every time.
const bulkMoves = computed(() => {
  const picked = selectedRows.value
  if (!picked.length) return []
  const shared = picked.reduce(
    (acc, row) => acc.filter((to) => (row.can_move_to ?? []).includes(to)),
    [...(picked[0].can_move_to ?? [])],
  )
  return shared.filter((to) => !picked.some((row) => NEEDS_A_NOTE(row.task.status, to)))
})

function toggle(row, event) {
  const next = new Set(selected.value)
  const ref = row.task.ref

  // Shift extends from the last one picked, within its own column. Across
  // columns a range has no meaning: the board is five lists, not one.
  if (event?.shiftKey && lastPicked.value) {
    const column = columns.value.find((c) => c.status === row.task.status)
    const refs = column.rows.map((r) => r.task.ref)
    const from = refs.indexOf(lastPicked.value)
    const to = refs.indexOf(ref)
    if (from !== -1 && to !== -1) {
      const [lo, hi] = from < to ? [from, to] : [to, from]
      refs.slice(lo, hi + 1).forEach((r) => next.add(r))
      selected.value = next
      lastPicked.value = ref
      return
    }
  }

  if (next.has(ref)) next.delete(ref)
  else next.add(ref)
  selected.value = next
  lastPicked.value = ref
}

function toggleColumn(column) {
  const next = new Set(selected.value)
  const refs = column.rows.map((r) => r.task.ref)
  const allPicked = refs.length > 0 && refs.every((r) => next.has(r))
  refs.forEach((r) => (allPicked ? next.delete(r) : next.add(r)))
  selected.value = next
}

function clear() {
  selected.value = new Set()
  lastPicked.value = null
  report.value = ''
}

async function moveSelection(to) {
  failure.value = ''
  report.value = ''
  busy.value = true
  const refs = [...selected.value]
  try {
    const results = await api.bulkTransition(refs, to)
    const moved = results.filter((r) => r.moved)
    const refused = results.filter((r) => !r.moved)

    // Refusals are named rather than counted. "1 refused" tells you something
    // went wrong and nothing about what to do; the server's own sentence is
    // the useful part and there is no reason to throw it away.
    report.value = refused.length
      ? `${moved.length} moved. ${refused.map((r) => `${r.ref}: ${r.error}`).join(' · ')}`
      : `${moved.length} ${moved.length === 1 ? 'task' : 'tasks'} moved to ${to}.`

    selected.value = new Set(refused.map((r) => r.ref))
    await load()
  } catch (err) {
    failure.value = err.message
  } finally {
    busy.value = false
  }
}

function open(row) {
  router.push(`/t/${row.task.ref}`)
}

// Escape clears a selection. It is the one gesture people try without being
// told, and without it the only way out is unpicking every card by hand.
function onKey(event) {
  if (event.key === 'Escape' && selected.value.size) clear()
}
onMounted(() => {
  load()
  window.addEventListener('keydown', onKey)
})
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <div v-if="!ready" />

  <div v-else class="board">
    <header class="head">
      <div>
        <h1>Board</h1>
        <p class="meta">
          {{ rows.length }} open {{ rows.length === 1 ? 'task' : 'tasks' }} across
          {{ projects.length }} {{ projects.length === 1 ? 'project' : 'projects' }}
          <template v-if="silentCount">
            <span class="sep">·</span>
            <span class="quiet">{{ silentCount }} gone quiet</span>
          </template>
        </p>
      </div>
    </header>

    <p v-if="failure" class="error">{{ failure }}</p>
    <p v-if="report" class="report">{{ report }}</p>

    <div v-if="!projects.length" class="empty">
      <p class="lead">Nothing here yet.</p>
      <p class="prose">
        A project holds tasks; a task holds a note for whoever picks it up. Make
        one, or register an agent and let it file the first one over MCP.
      </p>
      <div class="btns">
        <RouterLink to="/projects" class="btn btn-primary">Projects</RouterLink>
        <RouterLink to="/agents" class="btn btn-secondary">Register an agent</RouterLink>
      </div>
    </div>

    <div v-else class="columns">
      <section v-for="col in columns" :key="col.status" class="column" :data-status="col.status">
        <div class="col-head">
          <label class="all" :title="`Select every task in ${col.label}`">
            <input
              type="checkbox"
              :disabled="!col.rows.length"
              :checked="col.rows.length > 0 && col.rows.every((r) => selected.has(r.task.ref))"
              @change="toggleColumn(col)"
            />
          </label>
          <span class="col-name">{{ col.label }}</span>
          <span class="count mono">{{ col.rows.length }}</span>
        </div>

        <div class="stack">
          <TaskCard
            v-for="row in col.rows"
            :key="row.task.id"
            :row="row"
            :agents="agents"
            :selected="selected.has(row.task.ref)"
            @open="open(row)"
            @toggle="toggle(row, $event)"
          />
          <p v-if="!col.rows.length" class="none">{{ col.hint }}</p>
        </div>
      </section>
    </div>

    <!-- The bar appears only with a selection, and says what it will do to how
         many. Nothing here decides anything the workflow does not already
         allow: the moves are the server's own answer for these tasks. -->
    <div v-if="selected.size" class="bulk">
      <span class="n">{{ selected.size }} selected</span>
      <button
        v-for="to in bulkMoves"
        :key="to"
        class="btn btn-primary"
        :disabled="busy"
        @click="moveSelection(to)"
      >
        {{ MOVE_LABEL[to] ?? to }}
      </button>
      <span v-if="!bulkMoves.length" class="nomove">
        No move is available to all of these together.
      </span>
      <button class="btn btn-ghost" @click="clear">Clear</button>
    </div>
  </div>
</template>

<style scoped>
.board { display: flex; flex-direction: column; min-height: 100dvh; padding: var(--s-6); }

.head { display: flex; align-items: flex-end; justify-content: space-between; margin-bottom: var(--s-6); }
h1 { font-size: var(--t-xl); font-weight: 500; letter-spacing: -0.01em; }
.meta { font-size: 12.5px; color: var(--text-dim); margin-top: var(--s-2); }
.sep { margin: 0 var(--s-2); }
.quiet { color: var(--blocked); }

.report { font-size: 12.5px; color: var(--text-muted); margin-bottom: var(--s-4); }

/* Five columns that share the width and scroll independently. Equal fractions
   rather than content-sized, so the board does not reflow every time a task
   moves -- the columns are meant to be glanced at in the same place twice. */
.columns {
  flex: 1;
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: var(--s-4);
  min-height: 0;
}

.column {
  display: flex;
  flex-direction: column;
  min-height: 0;
  border: 1px solid var(--rule);
  border-radius: var(--r-md);
  background: color-mix(in srgb, var(--surface) 55%, transparent);
}

.col-head {
  display: flex;
  align-items: center;
  gap: var(--s-2);
  padding: var(--s-3);
  border-bottom: 1px solid var(--rule);
}
.col-name { font-size: var(--t-xs); text-transform: uppercase; letter-spacing: 0.08em; color: var(--text-muted); }
.count { margin-left: auto; font-size: var(--t-xs); color: var(--text-faint); }

.all { display: flex; }
.all input { accent-color: var(--accent); cursor: pointer; margin: 0; }
.all input:disabled { cursor: default; opacity: 0.3; }

/* The two columns that are the human's turn carry the accent on their header,
   the same mark a waiting row gets. Blocked borrows the amber it uses
   everywhere else. */
.column[data-status='review'] .col-head { box-shadow: inset 0 2px 0 var(--accent); }
.column[data-status='review'] .col-name { color: var(--accent); }
.column[data-status='blocked'] .col-head { box-shadow: inset 0 2px 0 var(--blocked); }
.column[data-status='blocked'] .col-name { color: var(--blocked); }

.stack {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: var(--s-2);
  padding: var(--s-3);
}
.none { font-size: var(--t-sm); color: var(--text-faint); line-height: 1.45; }

.bulk {
  position: sticky;
  bottom: 0;
  display: flex;
  align-items: center;
  gap: var(--s-3);
  margin-top: var(--s-4);
  padding: var(--s-3) var(--s-4);
  border-radius: var(--r-md);
  background: var(--surface-high);
  box-shadow: var(--e-2);
}
.n { font-size: 12.5px; color: var(--text-muted); margin-right: auto; }
.nomove { font-size: 12.5px; color: var(--text-faint); }

.empty { padding: var(--s-12) 0; max-width: 46ch; }
.lead { font-size: var(--t-lg); margin-bottom: var(--s-3); }
.prose { color: var(--text-muted); line-height: 1.6; margin-bottom: var(--s-6); }
.btns { display: flex; gap: var(--s-3); }

/* Columns become rows of their own below tablet. Five columns on a phone is
   five slivers; stacked, each one is still a readable list. */
@media (max-width: 1100px) {
  .columns { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .stack { overflow-y: visible; }
  .column { min-height: 140px; }
}
@media (max-width: 640px) {
  .columns { grid-template-columns: minmax(0, 1fr); }
  .bulk { flex-wrap: wrap; }
}
</style>
