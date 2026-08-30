<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '../api.js'
import TaskTable from '../components/TaskTable.vue'

// Every task across every project. Reached deliberately from the nav rather
// than sitting at the root, because a flat list of everything only makes sense
// once you know what the projects are.
const rows = ref([])
const projects = ref([])
const agents = ref([])
const failure = ref('')
const ready = ref(false)

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

onMounted(load)
</script>

<template>
  <div v-if="!ready" />
  <div v-else class="tasks">
    <p v-if="failure" class="error">{{ failure }}</p>

    <header>
      <div>
        <h1>Tasks</h1>
        <p class="counts">
          {{ rows.length }} {{ rows.length === 1 ? 'task' : 'tasks' }} across
          {{ projects.length }} {{ projects.length === 1 ? 'project' : 'projects' }}
          <!-- A readout, not a control: nothing to click, nothing to toggle. -->
          <template v-if="waiting">
            <span class="sep">·</span>
            <span class="waiting"><span class="dot" />{{ waiting }} waiting on you</span>
          </template>
        </p>
      </div>
      <span class="sort mono">most recently touched first</span>
    </header>

    <TaskTable v-if="rows.length" :rows="rows" :agents="agents" />

    <div v-else-if="!projects.length" class="empty">
      <p class="lead">No tasks anywhere yet.</p>
      <p class="prose">There are no projects either. Make one first.</p>
      <RouterLink to="/" class="btn btn-primary">Projects</RouterLink>
    </div>

    <div v-else class="empty">
      <p class="lead">No tasks yet.</p>
      <p class="prose">
        Open a project to file the first one, or let an agent file it over MCP.
      </p>
      <RouterLink to="/" class="btn btn-primary">Projects</RouterLink>
    </div>
  </div>
</template>

<style scoped>
.tasks { padding: var(--s-6) var(--s-6) var(--s-8); }

header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--s-4);
  margin-bottom: var(--s-8);
  padding-left: 14px;
}
h1 { font-size: var(--t-xl); font-weight: 500; letter-spacing: -0.01em; }
.counts { font-size: 12.5px; color: var(--text-dim); margin-top: var(--s-2); }
.sep { margin: 0 var(--s-2); }
.waiting { color: var(--accent); display: inline-flex; align-items: center; gap: 7px; }
.dot {
  width: 9px; height: 9px; border-radius: 50%;
  background: var(--accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 22%, transparent);
}
.sort { font-size: var(--t-sm); color: var(--text-faint); }

.empty { padding: var(--s-12) 14px; max-width: 54ch; }
.lead { font-size: var(--t-md); color: var(--text-muted); margin-bottom: var(--s-3); }
.prose { color: var(--text-dim); line-height: 1.6; margin-bottom: var(--s-6); text-wrap: pretty; }
</style>
