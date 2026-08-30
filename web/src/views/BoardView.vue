<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '../api.js'
import TaskTable from '../components/TaskTable.vue'

const rows = ref([])
const projects = ref([])
const agents = ref([])
const failure = ref('')
const ready = ref(false)

const creating = ref(false)
const draft = ref({ name: '', slug: '' })
const slugTouched = ref(false)
const createError = ref('')

const waiting = computed(() => rows.value.filter((r) => r.task.status === 'review').length)

async function load() {
  try {
    const [board, projectList, agentList] = await Promise.all([
      api.board(), api.projects(), api.agents(),
    ])
    rows.value = board
    projects.value = projectList
    agents.value = agentList
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
  const skeleton = clean[0] + clean.slice(1).replace(/[aeiou]/g, '')
  return skeleton.slice(0, 4)
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
    await api.createProject(draft.value.slug, draft.value.name)
    creating.value = false
    await load()
  } catch (err) {
    createError.value = err.message
  }
}

onMounted(load)
</script>

<template>
  <div v-if="!ready" />
  <div v-else class="board">
    <p v-if="failure" class="error">{{ failure }}</p>

    <div class="head">
      <div class="counts">
        <span>{{ rows.length }} {{ rows.length === 1 ? 'task' : 'tasks' }}</span>
        <!-- A readout, not a control: nothing to click, nothing to toggle. -->
        <template v-if="waiting">
          <span class="sep">·</span>
          <span class="waiting"><span class="dot" />{{ waiting }} waiting on you</span>
        </template>
      </div>

      <span class="sort mono">most recently touched first</span>
      <button v-if="!creating" class="btn btn-ghost" @click="openCreate">New project</button>
    </div>

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

    <TaskTable v-if="rows.length" :rows="rows" :agents="agents" />

    <!-- The only empty state that explains anything, because it is the only one
         a person sees before they understand the product. -->
    <div v-else-if="!projects.length" class="empty">
      <p class="lead">Nothing here yet.</p>
      <p class="prose">
        A project holds tasks; a task holds a note for whoever picks it up. Make
        one, or register an agent and let it file the first one over MCP.
      </p>
      <div class="row">
        <button class="btn btn-primary" @click="openCreate">New project</button>
        <RouterLink to="/agents" class="btn btn-secondary">Register an agent</RouterLink>
      </div>
    </div>

    <div v-else class="empty">
      <p class="lead">No tasks yet.</p>
      <p class="prose">
        Open a project to file the first one, or let an agent file it over MCP.
      </p>
    </div>
  </div>
</template>

<style scoped>
.board { padding: var(--s-6) var(--s-6) var(--s-8); }

.head {
  display: flex;
  align-items: center;
  gap: var(--s-4);
  padding: 0 var(--s-2) var(--s-4) 14px;
}

.counts { font-size: 12.5px; color: var(--text-dim); margin-right: auto; }
.sep { margin: 0 var(--s-2); }

.waiting { color: var(--accent); display: inline-flex; align-items: center; gap: 7px; }
.dot {
  width: 9px; height: 9px; border-radius: 50%;
  background: var(--accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 22%, transparent);
}

.sort { font-size: var(--t-sm); color: var(--text-faint); }

.newproject {
  background: var(--surface-raised);
  border-radius: var(--r-md);
  padding: var(--s-6);
  margin-bottom: var(--s-6);
}
.fields { display: flex; gap: var(--s-4); }
.grow { flex: 1; }
.prefix { width: 170px; }

.foot { display: flex; align-items: center; gap: var(--s-3); margin-top: var(--s-6); }
.note { margin-left: auto; font-size: var(--t-sm); color: var(--text-dim); }

.empty { padding: var(--s-12) 14px; max-width: 54ch; }
.lead { font-size: var(--t-md); color: var(--text-muted); margin-bottom: var(--s-3); }
.prose { color: var(--text-dim); line-height: 1.6; text-wrap: pretty; margin-bottom: var(--s-6); }
.row { display: flex; gap: var(--s-3); }
</style>
