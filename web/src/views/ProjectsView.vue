<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api.js'
import RelativeTime from '../components/RelativeTime.vue'

// The root screen. Projects first, because a task list across every project is
// only legible once you already know what the projects are -- and because a new
// install has projects before it has tasks.
const router = useRouter()

const projects = ref([])
const rows = ref([])
const failure = ref('')
const ready = ref(false)

const creating = ref(false)
const draft = ref({ name: '', slug: '' })
const slugTouched = ref(false)
const createError = ref('')

// Counts come from the board rather than a second endpoint: it already returns
// every task with its project, and this is a single-user tracker, not a
// dashboard over a million rows.
const summary = computed(() =>
  projects.value.map((p) => {
    const mine = rows.value.filter((r) => r.task.project === p.slug)
    const count = (status) => mine.filter((r) => r.task.status === status).length
    const touched = mine.reduce(
      (latest, r) => (!latest || r.task.updated_at > latest ? r.task.updated_at : latest),
      '',
    )
    return {
      ...p,
      total: mine.length,
      open: mine.filter((r) => r.task.status !== 'done').length,
      review: count('review'),
      blocked: count('blocked'),
      active: count('active'),
      touched,
    }
  }))

const waiting = computed(() => rows.value.filter((r) => r.task.status === 'review').length)

async function load() {
  try {
    const [projectList, board] = await Promise.all([api.projects(), api.board()])
    projects.value = projectList
    rows.value = board
  } catch (err) {
    failure.value = err.message
  } finally {
    ready.value = true
  }
}

// The prefix goes into every task reference this project will ever have, so it
// is derived as a convenience and then editable -- but only before creating.
function derivePrefix(name) {
  const clean = name.toLowerCase().replace(/[^a-z0-9]/g, '')
  if (!clean) return ''
  if (clean.length <= 8) return clean
  return (clean[0] + clean.slice(1).replace(/[aeiou]/g, '')).slice(0, 4)
}

function onName() {
  if (!slugTouched.value) draft.value.slug = derivePrefix(draft.value.name)
}

function openCreate() {
  creating.value = true
  draft.value = { name: '', slug: '' }
  slugTouched.value = false
  createError.value = ''
}

async function createProject() {
  createError.value = ''
  if (!/^[a-z0-9]{2,8}$/.test(draft.value.slug)) {
    createError.value = 'The reference prefix must be 2–8 lowercase letters or digits.'
    return
  }
  try {
    const project = await api.createProject(draft.value.slug, draft.value.name)
    creating.value = false
    router.push(`/p/${project.slug}`)
  } catch (err) {
    createError.value = err.message
  }
}

onMounted(load)
</script>

<template>
  <div v-if="!ready" />
  <div v-else class="projects">
    <p v-if="failure" class="error">{{ failure }}</p>

    <header>
      <div>
        <h1>Projects</h1>
        <p class="meta">
          {{ projects.length }} {{ projects.length === 1 ? 'project' : 'projects' }}
          <template v-if="waiting">
            <span class="sep">·</span>
            <RouterLink to="/tasks" class="waiting"><span class="dot" />{{ waiting }} waiting on you</RouterLink>
          </template>
        </p>
      </div>
      <button v-if="!creating" class="btn btn-primary" @click="openCreate">New project</button>
    </header>

    <form v-if="creating" class="newproject" @submit.prevent="createProject">
      <div class="fields">
        <div class="grow">
          <label class="field-label" for="pname">Project name</label>
          <input id="pname" v-model="draft.name" class="input input-lg" autofocus @input="onName" />
        </div>
        <div class="prefix">
          <label class="field-label" for="pslug">Reference prefix</label>
          <input
            id="pslug"
            v-model="draft.slug"
            class="input input-mono"
            @input="slugTouched = true; draft.slug = draft.slug.toLowerCase()"
          />
        </div>
      </div>

      <p v-if="createError" class="error">{{ createError }}</p>

      <div class="foot">
        <button class="btn btn-primary" type="submit" :disabled="!draft.name || !draft.slug">
          Create project
        </button>
        <button class="btn btn-secondary" type="button" @click="creating = false">Cancel</button>
        <span class="note">
          Tasks here will be numbered <code class="mono">{{ draft.slug || 'prefix' }}-1</code>,
          <code class="mono">{{ draft.slug || 'prefix' }}-2</code> …
          The prefix cannot be changed afterwards.
        </span>
      </div>
    </form>

    <ul v-if="summary.length" class="list">
      <li
        v-for="p in summary"
        :key="p.id"
        class="row rule-bottom"
        @click="router.push(`/p/${p.slug}`)"
      >
        <div class="name">
          <RouterLink :to="`/p/${p.slug}`" class="title" @click.stop>{{ p.name }}</RouterLink>
          <span class="slug mono">{{ p.slug }}</span>
        </div>

        <div class="signals">
          <span v-if="p.review" class="signal review"><span class="mark" />{{ p.review }} waiting on you</span>
          <span v-if="p.blocked" class="signal blocked"><span class="mark" />{{ p.blocked }} blocked</span>
          <span v-if="p.active" class="signal active"><span class="mark" />{{ p.active }} active</span>
        </div>

        <div class="counts">
          <span v-if="p.total" class="open">{{ p.open }} open</span>
          <span v-else class="none">no tasks yet</span>
          <span v-if="p.total" class="total">of {{ p.total }}</span>
        </div>

        <div class="ago">
          <RelativeTime v-if="p.touched" :value="p.touched" />
          <span v-else class="none">—</span>
        </div>
      </li>
    </ul>

    <div v-else class="empty">
      <p class="lead">Nothing here yet.</p>
      <p class="prose">
        A project holds tasks; a task holds a note for whoever picks it up. Make
        one, or register an agent and let it file the first one over MCP.
      </p>
      <div class="btns">
        <button class="btn btn-primary" @click="openCreate">New project</button>
        <RouterLink to="/agents" class="btn btn-secondary">Register an agent</RouterLink>
      </div>
    </div>
  </div>
</template>

<style scoped>
.projects { padding: var(--s-6) var(--s-6) var(--s-8); }

header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--s-4);
  margin-bottom: var(--s-8);
}
h1 { font-size: var(--t-xl); font-weight: 500; letter-spacing: -0.01em; }
.meta { font-size: 12.5px; color: var(--text-dim); margin-top: var(--s-2); }
.sep { margin: 0 var(--s-2); }

.waiting { color: var(--accent); display: inline-flex; align-items: center; gap: 7px; }
.waiting:hover { text-decoration: none; }
.dot {
  width: 9px; height: 9px; border-radius: 50%;
  background: var(--accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 22%, transparent);
}

/* A project row is much taller than a task row on purpose: this list is short,
   read once on arrival, and is the thing the whole app hangs off. */
.row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto 150px 64px;
  align-items: center;
  gap: var(--s-6);
  padding: var(--s-6) var(--s-2);
  cursor: pointer;
}
.row:hover {
  background:
    linear-gradient(to right,
      transparent, var(--rule) 48px,
      var(--rule) calc(100% - 48px), transparent) no-repeat bottom / 100% 1px,
    color-mix(in srgb, var(--text) 4%, transparent);
}

.name { display: flex; align-items: baseline; gap: var(--s-4); min-width: 0; }
.title {
  font-size: var(--t-lg);
  font-weight: 500;
  letter-spacing: -0.01em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.slug { font-size: var(--t-sm); color: var(--text-faint); flex: none; }

.signals { display: flex; gap: var(--s-4); font-size: 12.5px; flex-wrap: wrap; }
.signal { display: inline-flex; align-items: center; gap: 7px; white-space: nowrap; }
.mark { width: 8px; height: 8px; border-radius: 50%; flex: none; }
.review { color: var(--accent); }
.review .mark {
  background: var(--accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 22%, transparent);
}
.blocked { color: var(--blocked); }
.blocked .mark { border-radius: 0; background: var(--blocked); transform: rotate(45deg); }
.active { color: var(--text-muted); }
.active .mark { background: #968ae0; }

.counts { text-align: right; font-size: 13px; color: var(--text-muted); }
.total { color: var(--text-faint); margin-left: var(--s-2); }
.none { color: var(--text-faint); }

.ago { text-align: right; font-size: 12.5px; color: var(--text-dim); }

.newproject {
  background: var(--surface-raised);
  border-radius: var(--r-md);
  padding: var(--s-6);
  margin-bottom: var(--s-8);
}
.fields { display: flex; gap: var(--s-4); }
.grow { flex: 1; }
.prefix { width: 170px; }
.foot { display: flex; align-items: center; gap: var(--s-3); margin-top: var(--s-6); }
.note { margin-left: auto; font-size: var(--t-sm); color: var(--text-dim); }

.empty { padding: var(--s-12) var(--s-2); max-width: 54ch; }
.lead { font-size: var(--t-md); color: var(--text-muted); margin-bottom: var(--s-3); }
.prose { color: var(--text-dim); line-height: 1.6; margin-bottom: var(--s-6); text-wrap: pretty; }
.btns { display: flex; gap: var(--s-3); }

@media (max-width: 900px) {
  .row { grid-template-columns: minmax(0, 1fr) auto; row-gap: var(--s-3); }
  .counts, .ago { text-align: left; }
}
</style>
