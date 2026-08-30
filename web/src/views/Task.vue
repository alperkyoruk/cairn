<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api.js'
import { ago, stamp } from '../time.js'
import StatusBadge from '../components/StatusBadge.vue'
import StateBlock from '../components/StateBlock.vue'
import Worklog from '../components/Worklog.vue'

const props = defineProps({ ref: String })
const router = useRouter()

const detail = ref(null)
const failure = ref('')
const loading = ref(true)

// Which move the user has picked, if any. The buttons come from the server's
// can_move_to, so this screen never has to know the state machine.
const move = ref(null)
const form = ref(emptyForm())
const editing = ref(false)
const edit = ref({ title: '', body: '' })
const noting = ref(false)
const logging = ref(false)

function emptyForm() {
  return {
    where_i_left_off: '',
    next_step: '',
    blocked_on: '',
    what_was_tried: '',
    outcome: '',
  }
}

const needsBlocker = computed(() => move.value === 'blocked')

async function load() {
  loading.value = true
  try {
    detail.value = await api.task(props.ref)
    failure.value = ''
  } catch (err) {
    failure.value = err.message
  } finally {
    loading.value = false
  }
}

function start(to) {
  move.value = to
  form.value = emptyForm()
  // Carry the existing note forward so a small correction is a small edit.
  if (detail.value.state) {
    form.value.where_i_left_off = detail.value.state.where_i_left_off
    form.value.next_step = detail.value.state.next_step
  }
}

function stateFrom(f) {
  const any = f.where_i_left_off.trim() || f.next_step.trim() || f.blocked_on.trim()
  if (!any) return null
  return {
    where_i_left_off: f.where_i_left_off,
    next_step: f.next_step,
    blocked_on: f.blocked_on,
  }
}

function worklogFrom(f) {
  if (!f.what_was_tried.trim() && !f.outcome.trim()) return null
  return { what_was_tried: f.what_was_tried, outcome: f.outcome }
}

async function confirmMove() {
  failure.value = ''
  try {
    detail.value = await api.transition(props.ref, move.value, stateFrom(form.value), worklogFrom(form.value))
    move.value = null
  } catch (err) {
    failure.value = err.message
  }
}

async function saveNote() {
  failure.value = ''
  try {
    detail.value = await api.writeState(props.ref, {
      where_i_left_off: form.value.where_i_left_off,
      next_step: form.value.next_step,
      blocked_on: form.value.blocked_on,
    })
    noting.value = false
  } catch (err) {
    failure.value = err.message
  }
}

async function saveLog() {
  failure.value = ''
  try {
    detail.value = await api.appendWorklog(props.ref, {
      what_was_tried: form.value.what_was_tried,
      outcome: form.value.outcome,
    })
    logging.value = false
  } catch (err) {
    failure.value = err.message
  }
}

function startEdit() {
  edit.value = { title: detail.value.task.title, body: detail.value.task.body }
  editing.value = true
}

async function saveEdit() {
  failure.value = ''
  try {
    detail.value = await api.updateTask(props.ref, edit.value.title, edit.value.body)
    editing.value = false
  } catch (err) {
    failure.value = err.message
  }
}

async function remove() {
  const entries = detail.value.worklog.length
  const warning = `Delete ${detail.value.task.ref}?\n\n` +
    `Its ${entries} worklog ${entries === 1 ? 'entry goes' : 'entries go'} with it. ` +
    `This cannot be undone.`
  if (!confirm(warning)) return
  try {
    await api.deleteTask(props.ref)
    router.push(`/p/${detail.value.task.project}`)
  } catch (err) {
    failure.value = err.message
  }
}

function openNote() {
  noting.value = true
  form.value = emptyForm()
  if (detail.value.state) {
    form.value.where_i_left_off = detail.value.state.where_i_left_off
    form.value.next_step = detail.value.state.next_step
    form.value.blocked_on = detail.value.state.blocked_on
  }
}

onMounted(load)
watch(() => props.ref, load)
</script>

<template>
  <div v-if="loading" />
  <p v-else-if="!detail" class="error">{{ failure }}</p>

  <article v-else>
    <div class="crumbs mono faint">
      <RouterLink :to="`/p/${detail.task.project}`">{{ detail.task.project }}</RouterLink>
      <span>/</span>
      <span>{{ detail.task.ref }}</span>
    </div>

    <form v-if="editing" class="card stack edit" @submit.prevent="saveEdit">
      <div class="field">
        <label for="etitle">title</label>
        <input id="etitle" v-model="edit.title" />
      </div>
      <div class="field">
        <label for="ebody">body</label>
        <textarea id="ebody" v-model="edit.body" rows="6" />
      </div>
      <div class="row">
        <button class="primary" type="submit">save</button>
        <button type="button" @click="editing = false">cancel</button>
      </div>
    </form>

    <header v-else class="spread">
      <div>
        <h1>{{ detail.task.title }}</h1>
        <div class="row meta">
          <StatusBadge :status="detail.task.status" />
          <span class="faint" :title="stamp(detail.task.updated_at)">
            touched {{ ago(detail.task.updated_at) }} ago
          </span>
        </div>
      </div>
      <button @click="startEdit">edit</button>
    </header>

    <p v-if="detail.task.body && !editing" class="body">{{ detail.task.body }}</p>

    <p v-if="failure" class="error">{{ failure }}</p>

    <!-- The moves offered are exactly the ones the server says are legal for
         whoever is signed in. No disabled buttons, no guessing. -->
    <div class="moves row" v-if="!move">
      <button v-for="to in detail.can_move_to" :key="to" @click="start(to)">
        move to {{ to }}
      </button>
      <button v-if="!noting" @click="openNote">leave a note</button>
      <button v-if="!logging" @click="logging = true; form = emptyForm()">record an attempt</button>
      <button class="danger" @click="remove">delete</button>
    </div>

    <form v-if="move" class="card stack move" @submit.prevent="confirmMove">
      <p class="prompt">
        <StatusBadge :status="detail.task.status" /> → <StatusBadge :status="move" />
      </p>

      <div v-if="needsBlocker" class="field">
        <label for="blocked">what is blocking it — required</label>
        <textarea id="blocked" v-model="form.blocked_on" rows="2" autofocus />
      </div>

      <div class="field">
        <label for="where">where you left off</label>
        <textarea id="where" v-model="form.where_i_left_off" rows="2" />
      </div>
      <div class="field">
        <label for="next">next step</label>
        <textarea id="next" v-model="form.next_step" rows="2" />
      </div>
      <details>
        <summary class="faint">also record an attempt in the worklog</summary>
        <div class="field">
          <label for="tried">what was tried</label>
          <textarea id="tried" v-model="form.what_was_tried" rows="2" />
        </div>
        <div class="field">
          <label for="outcome">outcome</label>
          <textarea id="outcome" v-model="form.outcome" rows="2" />
        </div>
      </details>

      <div class="row">
        <button class="primary" type="submit">move to {{ move }}</button>
        <button type="button" @click="move = null">cancel</button>
      </div>
    </form>

    <form v-if="noting" class="card stack move" @submit.prevent="saveNote">
      <div class="field">
        <label for="nwhere">where you left off</label>
        <textarea id="nwhere" v-model="form.where_i_left_off" rows="2" autofocus />
      </div>
      <div class="field">
        <label for="nnext">next step</label>
        <textarea id="nnext" v-model="form.next_step" rows="2" />
      </div>
      <div class="field" v-if="detail.task.status === 'blocked'">
        <label for="nblocked">what is blocking it</label>
        <textarea id="nblocked" v-model="form.blocked_on" rows="2" />
      </div>
      <div class="row">
        <button class="primary" type="submit">save note</button>
        <button type="button" @click="noting = false">cancel</button>
      </div>
    </form>

    <form v-if="logging" class="card stack move" @submit.prevent="saveLog">
      <div class="field">
        <label for="ltried">what was tried</label>
        <textarea id="ltried" v-model="form.what_was_tried" rows="2" autofocus />
      </div>
      <div class="field">
        <label for="loutcome">outcome</label>
        <textarea id="loutcome" v-model="form.outcome" rows="2" />
      </div>
      <div class="row">
        <button class="primary" type="submit">record</button>
        <button type="button" @click="logging = false">cancel</button>
      </div>
    </form>

    <StateBlock :state="detail.state" />
    <Worklog :entries="detail.worklog" />
  </article>
</template>

<style scoped>
.crumbs { display: flex; gap: var(--space-2); font-size: var(--text-sm); margin-bottom: var(--space-3); }

header { align-items: flex-start; margin-bottom: var(--space-4); }
h1 { margin: 0 0 var(--space-2); font-size: var(--text-lg); letter-spacing: -0.01em; }
.meta { font-size: var(--text-sm); }

.body {
  white-space: pre-wrap;
  color: var(--ink-muted);
  margin: 0 0 var(--space-6);
  max-width: 46rem;
}

.moves { flex-wrap: wrap; margin-bottom: var(--space-6); }
.move { margin-bottom: var(--space-6); max-width: 46rem; }
.prompt { margin: 0; font-size: var(--text-sm); }

details summary { cursor: pointer; font-size: var(--text-sm); }
details .field { margin-top: var(--space-3); }

.edit { margin-bottom: var(--space-6); max-width: 46rem; }

article > section { margin-bottom: var(--space-6); max-width: 46rem; }
</style>
