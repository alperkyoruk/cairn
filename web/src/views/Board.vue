<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api.js'
import { ago, stamp } from '../time.js'
import StatusBadge from '../components/StatusBadge.vue'

// The root screen: every task across every project, most recently touched
// first. Read-only, and no filters -- if you are looking for something
// specific, you already know which project it is in.
const rows = ref([])
const projects = ref([])
const failure = ref('')
const loading = ref(true)

const creating = ref(false)
const draft = ref({ slug: '', name: '' })

async function load() {
  try {
    const [board, list] = await Promise.all([api.board(), api.projects()])
    rows.value = board
    projects.value = list
  } catch (err) {
    failure.value = err.message
  } finally {
    loading.value = false
  }
}

async function createProject() {
  failure.value = ''
  try {
    await api.createProject(draft.value.slug, draft.value.name)
    draft.value = { slug: '', name: '' }
    creating.value = false
    await load()
  } catch (err) {
    failure.value = err.message
  }
}

onMounted(load)
</script>

<template>
  <div v-if="loading" />

  <div v-else>
    <p v-if="failure" class="error">{{ failure }}</p>

    <div v-if="!projects.length && !rows.length" class="empty">
      <p>Nothing here yet. A project is the first stone.</p>
      <button class="primary" @click="creating = true" v-if="!creating">new project</button>
    </div>

    <form v-if="creating" class="card newproject" @submit.prevent="createProject">
      <div class="row">
        <input v-model="draft.slug" placeholder="slug (cairn)" class="slug" />
        <input v-model="draft.name" placeholder="name (Cairn)" />
        <button class="primary" type="submit">create</button>
        <button type="button" @click="creating = false">cancel</button>
      </div>
      <p class="faint hint">The slug is what task references are built from: cairn-12.</p>
    </form>

    <table v-if="rows.length">
      <thead>
        <tr>
          <th class="c-project">project</th>
          <th>task</th>
          <th class="c-status">status</th>
          <th class="c-next">next step</th>
          <th class="c-who">last updated by</th>
          <th class="c-ago"></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in rows" :key="row.task.id">
          <td class="c-project">
            <RouterLink :to="`/p/${row.task.project}`" class="mono muted">{{ row.task.project }}</RouterLink>
          </td>
          <td class="title">
            <RouterLink :to="`/t/${row.task.ref}`">{{ row.task.title }}</RouterLink>
          </td>
          <td class="c-status"><StatusBadge :status="row.task.status" /></td>
          <td class="c-next">
            <span v-if="row.state?.next_step">{{ row.state.next_step }}</span>
            <span v-else class="faint">—</span>
          </td>
          <td class="c-who muted">{{ row.state?.updated_by ?? '—' }}</td>
          <td class="c-ago faint" :title="stamp(row.task.updated_at)">{{ ago(row.task.updated_at) }}</td>
        </tr>
      </tbody>
    </table>

    <div v-if="projects.length" class="footer spread">
      <div class="row projects">
        <span class="faint">projects</span>
        <RouterLink v-for="p in projects" :key="p.id" :to="`/p/${p.slug}`" class="mono">{{ p.slug }}</RouterLink>
      </div>
      <button v-if="!creating" @click="creating = true">new project</button>
    </div>
  </div>
</template>

<style scoped>
/* table-layout: fixed is load-bearing: without it a long next_step widens its
   column to fit, the table overflows its container, and the "how long ago"
   column falls off the right edge. Fixed layout is also what makes the
   ellipsis truncation below work at all. */
table {
  width: 100%;
  table-layout: fixed;
  border-collapse: collapse;
  font-size: var(--text-sm);
}

th {
  text-align: left;
  font-weight: 400;
  font-size: var(--text-xs);
  color: var(--ink-faint);
  padding: 0 var(--space-3) var(--space-2);
  border-bottom: 1px solid var(--border);
}

td {
  padding: var(--space-2) var(--space-3);
  border-bottom: 1px solid var(--border);
  vertical-align: top;
}

tbody tr:hover { background: var(--bg-hover); }

.title { font-size: var(--text-base); }
.c-project { width: 7rem; }
.c-status { width: 6rem; }
.c-next { width: 32%; color: var(--ink-muted); }
.c-who { width: 9rem; }
.c-ago { width: 4rem; text-align: right; white-space: nowrap; }

.c-project a, .c-who, .c-next span, .title a {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.newproject { margin-bottom: var(--space-6); }
.slug { max-width: 12rem; }
.hint { font-size: var(--text-xs); margin: var(--space-2) 0 0; }

.footer { margin-top: var(--space-6); font-size: var(--text-sm); }
.projects { gap: var(--space-3); flex-wrap: wrap; }
</style>
