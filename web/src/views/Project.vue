<script setup>
import { onMounted, ref, watch } from 'vue'
import { api } from '../api.js'
import { ago, stamp } from '../time.js'
import StatusBadge from '../components/StatusBadge.vue'

const props = defineProps({ slug: String })

const project = ref(null)
const tasks = ref([])
const failure = ref('')
const loading = ref(true)
const draft = ref({ title: '', body: '', status: 'backlog' })
const adding = ref(false)

async function load() {
  loading.value = true
  failure.value = ''
  try {
    project.value = await api.project(props.slug)
    tasks.value = await api.projectTasks(props.slug)
  } catch (err) {
    failure.value = err.message
  } finally {
    loading.value = false
  }
}

async function addTask() {
  failure.value = ''
  try {
    await api.createTask(project.value.id, draft.value.title, draft.value.body, draft.value.status)
    draft.value = { title: '', body: '', status: 'backlog' }
    adding.value = false
    await load()
  } catch (err) {
    failure.value = err.message
  }
}

onMounted(load)
watch(() => props.slug, load)
</script>

<template>
  <div v-if="loading" />
  <div v-else-if="!project"><p class="error">{{ failure }}</p></div>

  <div v-else>
    <div class="spread head">
      <div>
        <h1>{{ project.name }}</h1>
        <p class="mono faint">{{ project.slug }}</p>
      </div>
      <button v-if="!adding" class="primary" @click="adding = true">new task</button>
    </div>

    <p v-if="failure" class="error">{{ failure }}</p>

    <form v-if="adding" class="card stack" @submit.prevent="addTask">
      <div class="field">
        <label for="title">title</label>
        <input id="title" v-model="draft.title" autofocus />
      </div>
      <div class="field">
        <label for="body">body</label>
        <textarea id="body" v-model="draft.body" />
      </div>
      <div class="row">
        <select v-model="draft.status">
          <option value="backlog">into backlog</option>
          <option value="queue">into queue — ready to work on</option>
        </select>
        <button class="primary" type="submit">create</button>
        <button type="button" @click="adding = false">cancel</button>
      </div>
    </form>

    <p v-if="!tasks.length" class="empty">No tasks in this project yet.</p>

    <table v-else>
      <tbody>
        <tr v-for="task in tasks" :key="task.id">
          <td class="c-ref mono faint">{{ task.ref }}</td>
          <td class="title"><RouterLink :to="`/t/${task.ref}`">{{ task.title }}</RouterLink></td>
          <td class="c-status"><StatusBadge :status="task.status" /></td>
          <td class="c-ago faint" :title="stamp(task.updated_at)">{{ ago(task.updated_at) }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.head { margin-bottom: var(--space-6); align-items: flex-start; }
h1 { margin: 0; font-size: var(--text-xl); letter-spacing: -0.02em; }
.head p { margin: 0; font-size: var(--text-sm); }

form { margin-bottom: var(--space-6); }
select {
  font: inherit;
  color: var(--ink);
  background: var(--bg-raised);
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-sm);
  padding: var(--space-1) var(--space-2);
}

table { width: 100%; table-layout: fixed; border-collapse: collapse; font-size: var(--text-sm); }
td { padding: var(--space-2) var(--space-3); border-bottom: 1px solid var(--border); }
tbody tr:hover { background: var(--bg-hover); }
.c-ref { width: 6rem; }
.c-status { width: 5.5rem; }
.c-ago { width: 3.5rem; text-align: right; white-space: nowrap; }
.title { font-size: var(--text-base); }
.title a { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
