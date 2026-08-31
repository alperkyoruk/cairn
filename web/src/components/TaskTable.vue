<script setup>
import { useRouter } from 'vue-router'
import StatusMark from './StatusMark.vue'
import ActorName from './ActorName.vue'
import RelativeTime from './RelativeTime.vue'
import { formatFull } from '../composables/useRelativeTime.js'
import { isSilent } from '../silence.js'

// The board and project detail are the same table; the project column is the
// only difference between them.
const props = defineProps({
  rows: { type: Array, required: true },
  showProject: { type: Boolean, default: true },
  agents: { type: Array, default: () => [] },
})

const router = useRouter()

function isAgent(name) {
  return props.agents.some((a) => a.name === name)
}

// Words for a missing record, a dash for an empty field, and two cases where
// the cell is borrowed by whichever record actually has something to say.
//
// On a blocked task next_step is usually empty and blocked_on holds the only
// sentence that matters. And when the last thing to happen was an attempt
// recorded without moving the task -- an agent reviewing work it did not do,
// most often -- next_step still says what the previous writer intended, which
// is not what just happened. Both borrow the column: same width, same header,
// and the hue is what says the field is different.
function nextCell(row) {
  if (row.attempt) {
    return { kind: 'attempt', text: row.attempt.what_was_tried }
  }
  const state = row.state
  if (!state) return { kind: 'missing', text: 'no state yet' }
  if (row.task.status === 'blocked' && state.blocked_on) {
    return { kind: 'blocked', text: state.blocked_on }
  }
  if (!state.next_step) return { kind: 'empty', text: '—' }
  return { kind: 'value', text: state.next_step }
}

// Who caused the most recent event, not who last wrote the note. Those were the
// same person until an agent could review someone else's task; after that the
// row rose to the top of the board still crediting the previous writer.
function lastActor(row) {
  return row.attempt ? row.attempt.actor : row.state?.updated_by
}

// Recency by luminance: anything touched under an hour ago stays bright, older
// rows recede. It costs nothing and the top of the board glows slightly.
function isRecent(row) {
  return Date.now() - new Date(row.task.updated_at).getTime() < 3600_000
}

function open(row) {
  router.push(`/t/${row.task.ref}`)
}
</script>

<template>
  <table>
    <thead>
      <tr>
        <th class="c-edge"></th>
        <th v-if="showProject" class="c-project">Project</th>
        <th class="c-task">Task</th>
        <th class="c-status">Status</th>
        <th class="c-next">Next step</th>
        <th class="c-by">Last updated by</th>
        <th class="c-ago">Ago</th>
      </tr>
    </thead>
    <tbody>
      <tr
        v-for="row in rows"
        :key="row.task.id"
        class="row rule-bottom"
        :class="{
          waiting: row.task.status === 'review',
          silent: isSilent(row.task),
          stale: !isRecent(row),
          done: row.task.status === 'done',
        }"
        @click="open(row)"
      >
        <td class="c-edge"></td>

        <td v-if="showProject" class="c-project">
          <RouterLink :to="`/p/${row.task.project}`" @click.stop>{{ row.task.project }}</RouterLink>
        </td>

        <td class="c-task">
          <span :title="row.task.title">{{ row.task.title }}</span>
        </td>

        <td class="c-status"><StatusMark :status="row.task.status" /></td>

        <td class="c-next">
          <span :class="`next-${nextCell(row).kind}`" :title="nextCell(row).text">
            {{ nextCell(row).text }}
          </span>
        </td>

        <td class="c-by">
          <ActorName :name="lastActor(row)" :is-agent="isAgent(lastActor(row))" />
        </td>

        <!-- On a silent row this number has the opposite meaning to everywhere
             else: not "recently touched" but "nothing has happened for this
             long". Same value, so the hue carries the difference. -->
        <td class="c-ago" :title="isSilent(row.task) ? 'Active, but nothing written for ' + formatFull(row.task.updated_at) : null">
          <RelativeTime :value="row.task.updated_at" />
        </td>
      </tr>
    </tbody>
  </table>
</template>

<style scoped>
table {
  width: 100%;
  table-layout: fixed;
  border-collapse: collapse;
}

th {
  text-align: left;
  font-weight: 400;
  font-size: var(--t-xs);
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: color-mix(in srgb, var(--text) 60%, transparent);
  padding: 0 var(--s-2) var(--s-2);
}

/* Rows land at 30px so a full board fits one screen without scrolling. The
   five-second glance is the whole use case; do not loosen this. */
td {
  padding: var(--s-2);
  height: 30px;
  vertical-align: middle;
}

.c-edge { width: 14px; padding: 0; }
.c-project { width: 110px; font-size: 12.5px; color: var(--text-muted); }
.c-task { font-size: var(--t-base); color: var(--text); }
.c-status { width: 118px; }
.c-next { width: 330px; font-size: var(--t-base); color: var(--text-muted); }
.c-by { width: 104px; font-size: 12.5px; }
.c-ago { width: 56px; text-align: right; font-size: 12.5px; color: var(--text-dim); }

/* The fixed column widths add up to more than a phone is wide, and with
   table-layout: fixed that made the page itself scroll sideways -- on three
   screens, including the root. Columns drop in order of how easily the reader
   can reconstruct them by opening the task.
   
   Who touched it and when go first: both are one click away and neither
   answers "does this need me". The project goes last and only when there is
   genuinely no room, because on a cross-project board it is real context. What
   never goes is the title, the status, and the next step -- on a blocked row
   that last cell is carrying the blocker, which is the only sentence that
   matters on it. */
@media (max-width: 900px) {
  .c-by, .c-ago { display: none; }
  .c-project { width: 84px; }
  .c-status { width: 100px; }
  .c-next { width: auto; }
}

@media (max-width: 560px) {
  .c-project { display: none; }
  .c-status { width: 92px; }
}

/* Never two lines: a row that can grow breaks the scan rhythm, and the full
   text is one click away. */
.c-task span, .c-next span, .c-project a {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.row { cursor: pointer; }
.row:hover {
  background:
    linear-gradient(to right,
      transparent, var(--rule) 48px,
      var(--rule) calc(100% - 48px), transparent) no-repeat bottom / 100% 1px,
    color-mix(in srgb, var(--text) 4%, transparent);
}

/* review = waiting on you, without a filter. A 2px mark at the row's left edge
   survives peripheral vision at the far edge of a wide monitor. */
.row.waiting .c-edge { box-shadow: inset 2px 0 0 var(--accent); }

.row.stale .c-by, .row.stale .c-ago { color: var(--text-dim); }
.row.done .c-task { color: var(--text-muted); }
.row.done .c-project { color: var(--text-dim); }

.next-missing { font-size: var(--t-sm); color: var(--text-faint); }
.next-empty { color: var(--text-faint); }
.next-blocked { color: color-mix(in srgb, var(--blocked) 76%, #75798c); }
/* Three treatments for one column, and the hue is the whole distinction: muted
   is the routine next step, amber is a blocker, accent is something recorded
   just now that nobody has read. Accent is already the "this involves you" hue
   on review rows, which is what an unaccompanied attempt usually is. */
.next-attempt { color: color-mix(in srgb, var(--accent) 88%, #75798c); }

/* Silence reads as a blocker, because that is what it is: work that is not
   happening and will not resume on its own. It borrows the blocked hue rather
   than inventing a fourth, and it overrides .stale, which would otherwise dim
   the very cell that carries the point. */
.row.silent .c-ago { color: var(--blocked); }
.row.silent .c-edge { box-shadow: inset 2px 0 0 var(--blocked); }
</style>
