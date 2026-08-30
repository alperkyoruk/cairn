<script setup>
import { useRouter } from 'vue-router'
import StatusMark from './StatusMark.vue'
import ActorName from './ActorName.vue'
import RelativeTime from './RelativeTime.vue'
import { formatFull } from '../composables/useRelativeTime.js'

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

// Three distinct cases, and the distinction is deliberate: words for a missing
// record, a dash for an empty field.
//
// The third is the only cell in the app whose meaning depends on the row's
// status. On a blocked task next_step is usually empty and blocked_on holds the
// only sentence that matters, so the cell borrows the column -- same width, same
// header, and the hue is what says the field is different.
function nextCell(row) {
  const state = row.state
  if (!state) return { kind: 'missing', text: 'no state yet' }
  if (row.task.status === 'blocked' && state.blocked_on) {
    return { kind: 'blocked', text: state.blocked_on }
  }
  if (!state.next_step) return { kind: 'empty', text: '—' }
  return { kind: 'value', text: state.next_step }
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
        :class="{ waiting: row.task.status === 'review', stale: !isRecent(row), done: row.task.status === 'done' }"
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
          <ActorName :name="row.state?.updated_by" :is-agent="isAgent(row.state?.updated_by)" />
        </td>

        <td class="c-ago">
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
</style>
