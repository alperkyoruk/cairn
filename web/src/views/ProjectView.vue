<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api.js'
import TaskTable from '../components/TaskTable.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'

const props = defineProps({ slug: String })
const router = useRouter()

const project = ref(null)
const rows = ref([])
const agents = ref([])
const failure = ref('')
const ready = ref(false)

const adding = ref(false)
const draft = ref({ title: '', body: '' })
const confirming = ref(false)
const blocker = ref('')

const blocked = computed(() => rows.value.filter((r) => r.task.status === 'blocked').length)

async function load() {
  ready.value = false
  try {
    // Three requests, whatever the project holds. The task list answers in the
    // same {task, state} shape the board does, so there is nothing to assemble.
    const [p, taskRows, agentList] = await Promise.all([
      api.project(props.slug), api.projectTasks(props.slug), api.agents(),
    ])
    project.value = p
    agents.value = agentList
    rows.value = taskRows
  } catch (err) {
    failure.value = err.message
  } finally {
    ready.value = true
  }
}

async function addTask() {
  failure.value = ''
  try {
    await api.createTask(project.value.id, draft.value.title, draft.value.body, 'backlog')
    draft.value = { title: '', body: '' }
    adding.value = false
    await load()
  } catch (err) {
    failure.value = err.message
  }
}

// Deleting a project that still holds tasks is refused by the server rather
// than cascading, so there is no point opening a confirmation for it: say why
// instead, next to the button that failed.
function askDelete() {
  blocker.value = ''
  if (rows.value.length) {
    const n = rows.value.length
    blocker.value = `${project.value.slug} still holds ${n} ${n === 1 ? 'task' : 'tasks'}. `
      + `Delete them first — Cairn will not take a project's work with it.`
    return
  }
  confirming.value = true
}

async function removeProject() {
  try {
    await api.deleteProject(project.value.slug)
    router.push('/')
  } catch (err) {
    failure.value = err.message
    confirming.value = false
  }
}

onMounted(load)
watch(() => props.slug, load)
</script>

<template>
  <div v-if="!ready" />
  <div v-else-if="!project" class="pad"><p class="error">{{ failure }}</p></div>

  <div v-else class="project">
    <nav class="crumbs mono">
      <RouterLink to="/">projects</RouterLink>
      <span class="faint">/</span>
      <span class="dim">{{ project.slug }}</span>
    </nav>

    <header>
      <div>
        <h1>{{ project.name }}</h1>
        <p class="meta">
          {{ rows.length }} {{ rows.length === 1 ? 'task' : 'tasks' }}
          <template v-if="blocked">
            <span class="sep">·</span>
            <span class="blocked"><span class="diamond" />{{ blocked }} blocked</span>
          </template>
        </p>
      </div>
      <button v-if="!adding" class="btn btn-primary" @click="adding = true">New task</button>
    </header>

    <p v-if="failure" class="error">{{ failure }}</p>

    <!-- In place, not behind a modal: it is a title and a body, and it is
         faster to type than to open. -->
    <form v-if="adding" class="newtask" @submit.prevent="addTask">
      <input v-model="draft.title" class="input input-lg" placeholder="Title" autofocus />
      <textarea
        v-model="draft.body"
        class="input"
        rows="3"
        placeholder="Body — what the next reader needs to know before they start"
      />
      <div class="foot">
        <button class="btn btn-primary" type="submit" :disabled="!draft.title">Create in backlog</button>
        <button class="btn btn-secondary" type="button" @click="adding = false">Cancel</button>
        <span class="note mono">new tasks always start in backlog</span>
      </div>
    </form>

    <TaskTable v-if="rows.length" :rows="rows" :show-project="false" :agents="agents" />

    <div v-else class="empty">
      <p class="lead">No tasks in {{ project.slug }}.</p>
      <p class="prose">Everything you file starts in backlog and waits for you to queue it.</p>
      <button class="btn btn-primary" @click="adding = true">New task</button>
    </div>

    <footer class="rule-top">
      <button class="btn btn-ghost btn-danger" @click="askDelete">Delete project</button>
      <p v-if="blocker" class="blocker">{{ blocker }}</p>
    </footer>

    <ConfirmDialog
      :open="confirming"
      :title="`Delete ${project.slug}?`"
      :confirm-word="project.slug"
      @cancel="confirming = false"
      @confirm="removeProject"
    >
      This project holds no tasks, so nothing is destroyed with it. The
      reference prefix <code class="mono">{{ project.slug }}</code> becomes
      available again afterwards.
    </ConfirmDialog>
  </div>
</template>

<style scoped>
.project, .pad { padding: var(--s-6) var(--s-6) var(--s-8); }

.crumbs { display: flex; gap: var(--s-2); font-size: var(--t-sm); margin-bottom: var(--s-4); }
.crumbs a { color: var(--text-dim); }

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
.blocked { color: var(--blocked); display: inline-flex; align-items: center; gap: 7px; }
.diamond { width: 8px; height: 8px; background: var(--blocked); transform: rotate(45deg); }

.newtask {
  display: flex;
  flex-direction: column;
  gap: var(--s-3);
  background: var(--surface-raised);
  border-radius: var(--r-md);
  padding: var(--s-6);
  margin-bottom: var(--s-6);
}
.newtask textarea { min-height: 64px; }
.foot { display: flex; align-items: center; gap: var(--s-3); }
.note { margin-left: auto; font-size: var(--t-sm); color: var(--text-dim); }

footer { margin-top: var(--s-12); padding-top: var(--s-6); }
.blocker { font-size: var(--t-sm); color: var(--blocked); margin-top: var(--s-2); max-width: 60ch; }

.empty { padding: var(--s-12) 14px; max-width: 54ch; }
.lead { font-size: var(--t-md); color: var(--text-muted); margin-bottom: var(--s-3); }
.prose { color: var(--text-dim); line-height: 1.6; margin-bottom: var(--s-6); }
</style>
