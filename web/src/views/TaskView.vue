<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api.js'
import StatusMark from '../components/StatusMark.vue'
import StatePanel from '../components/StatePanel.vue'
import Worklog from '../components/Worklog.vue'
import TransitionActions from '../components/TransitionActions.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import RelativeTime from '../components/RelativeTime.vue'
import { formatAbsolute } from '../composables/useRelativeTime.js'

const props = defineProps({ taskRef: String })
const router = useRouter()

const detail = ref(null)
const agents = ref([])
const failure = ref('')
const ready = ref(false)
const busy = ref(false)

// Two moves are refused without a value, so those two buttons ask before they
// act. The server is the authority -- internal/workflow's Requires decides
// this, and a refusal comes back inline if the list below ever drifts from it.
const pending = ref(null)          // the status being moved to, once a form is open
const form = ref({ finding: '', where_i_left_off: '', next_step: '', blocked_on: '' })

// blocked needs a reason. Sending work back from review needs to say what was
// wrong with it, or the agent picks the task up reading its own last note.
function fieldsFor(to) {
  const rejecting = to === 'active' && detail.value?.task.status === 'review'
  if (to !== 'blocked' && !rejecting) return []

  // An omitted field keeps what is stored -- but a task with no note yet has
  // nothing to inherit, and the server refuses a half-written one. Without this
  // the form offers no field that could satisfy its own refusal: a task the
  // human moved to active themselves, leaving no note, could never be blocked.
  const whole = detail.value?.state ? [] : ['where_i_left_off', 'next_step']
  if (to === 'blocked') return [...whole, 'blocked_on']
  return whole.length ? ['finding', ...whole] : ['finding', 'next_step']
}

// Where each field lands. A rejection's finding goes to the worklog, not to
// state: state holds one note that is overwritten in place, so a reason written
// there is erased by the next agent's checkpoint -- and on the way in it lands
// on top of where_i_left_off, which is the agent's own account of what it did.
// The worklog is the record that keeps.
const WORKLOG_FIELDS = new Set(['finding'])

const PROMPTS = {
  finding: 'What you found',
  where_i_left_off: 'Where the work stands',
  next_step: 'What needs to be different',
  blocked_on: 'What is blocking it',
}

const PLACEHOLDERS = {
  finding: 'What you saw when you reviewed it',
  where_i_left_off: 'What has been done so far, if anything',
  next_step: 'The single next thing the agent should do',
  blocked_on: 'Exactly what is needed in order to continue',
}

const editing = ref(false)
const edit = ref({ title: '', body: '' })
const confirming = ref(false)

const blockedSince = computed(() => {
  if (detail.value?.task.status !== 'blocked') return ''
  const entry = [...(detail.value.worklog ?? [])].reverse()
    .find((e) => e.to_status === 'blocked')
  return entry ? formatAbsolute(entry.created_at) : ''
})

async function load() {
  ready.value = false
  try {
    const [task, agentList] = await Promise.all([api.task(props.taskRef), api.agents()])
    detail.value = task
    agents.value = agentList
    failure.value = ''
  } catch (err) {
    failure.value = err.message
  } finally {
    ready.value = true
  }
}

function startMove(to) {
  if (fieldsFor(to).length && pending.value !== to) {
    pending.value = to
    form.value = { finding: '', where_i_left_off: '', next_step: '', blocked_on: '' }
    return
  }
  move(to)
}

// The human's own worklog entry, written without moving the task.
//
// Every sentence the human wrote used to go to state, the one record designed
// to be overwritten, and the append-only trail held only status arrows from
// them. cairn-2 fixed that for a rejection; this is the rest of it, and it is
// the same act an agent performs when it reviews work it did not do.
//
// Deliberately NOT a state edit. Letting the human write state outside a move
// would put back the clobbering that cairn-2 removed: state is one note, and
// the agent's account of what it did lives in it.
const recording = ref(false)
const record = ref({ what_was_tried: '', outcome: '' })

async function appendWorklog() {
  failure.value = ''
  busy.value = true
  try {
    detail.value = await api.appendWorklog(props.taskRef, { ...record.value })
    record.value = { what_was_tried: '', outcome: '' }
    recording.value = false
  } catch (err) {
    failure.value = err.message
  } finally {
    busy.value = false
  }
}

async function move(to) {
  failure.value = ''
  busy.value = true
  try {
    const needs = fieldsFor(to)
    // Only what this move asked for is sent. An omitted field keeps whatever is
    // already stored, so sending work back no longer overwrites the agent's
    // where_i_left_off with a review comment.
    const stateFields = needs.filter((f) => !WORKLOG_FIELDS.has(f))
    const state = stateFields.length
      ? Object.fromEntries(stateFields.map((f) => [f, form.value[f]]))
      : null
    const worklog = needs.includes('finding')
      ? { what_was_tried: form.value.finding }
      : null
    // The status mark changing is the feedback; there is no toast.
    detail.value = await api.transition(props.taskRef, to, state, worklog)
    pending.value = null
  } catch (err) {
    failure.value = err.message
  } finally {
    busy.value = false
  }
}

const canSubmit = computed(() =>
  fieldsFor(pending.value).every((f) => form.value[f].trim()))

function startEdit() {
  edit.value = { title: detail.value.task.title, body: detail.value.task.body }
  editing.value = true
}

async function saveEdit() {
  failure.value = ''
  try {
    detail.value = await api.updateTask(props.taskRef, edit.value.title, edit.value.body)
    editing.value = false
  } catch (err) {
    failure.value = err.message
  }
}

async function remove() {
  busy.value = true
  try {
    const project = detail.value.task.project
    await api.deleteTask(props.taskRef)
    router.push(`/p/${project}`)
  } catch (err) {
    failure.value = err.message
    confirming.value = false
  } finally {
    busy.value = false
  }
}

onMounted(load)
watch(() => props.taskRef, load)
</script>

<template>
  <div v-if="!ready" />
  <div v-else-if="!detail" class="pad"><p class="error">{{ failure }}</p></div>

  <div v-else class="task">
    <div class="content">
      <nav class="crumbs mono">
        <RouterLink :to="`/p/${detail.task.project}`">{{ detail.task.project }}</RouterLink>
        <span class="faint">/</span>
        <span class="dim">{{ detail.task.ref }}</span>
      </nav>

      <div class="statusline">
        <StatusMark :status="detail.task.status" />
        <span v-if="blockedSince" class="since mono">since {{ blockedSince }}</span>
      </div>

      <form v-if="editing" class="edit" @submit.prevent="saveEdit">
        <input v-model="edit.title" class="input input-lg" />
        <textarea v-model="edit.body" class="input" rows="6" />
        <div class="row">
          <button class="btn btn-primary" type="submit">Save</button>
          <button class="btn btn-secondary" type="button" @click="editing = false">Cancel</button>
        </div>
      </form>

      <template v-else>
        <h1>{{ detail.task.title }}</h1>
        <p v-if="detail.task.body" class="body">{{ detail.task.body }}</p>
      </template>

      <p v-if="failure" class="error">{{ failure }}</p>

      <StatePanel :state="detail.state" :status="detail.task.status" :agents="agents" />
      <Worklog :entries="detail.worklog" :agents="agents" />

      <!-- Under the worklog, because it appends to it. A separate act rather
           than fields hung off every move: the moves are the fast path and
           should stay fast, and most of them have nothing to record. -->
      <button v-if="!recording" class="btn btn-ghost record" @click="recording = true">
        Record what you found
      </button>

      <form v-else class="record-form" @submit.prevent="appendWorklog">
        <div class="field">
          <label class="field-label" for="w-tried">What you did</label>
          <textarea
            id="w-tried"
            v-model="record.what_was_tried"
            class="input"
            rows="2"
            placeholder="What you actually did, in enough detail that someone repeating it would recognise it"
            autofocus
          />
        </div>
        <div class="field">
          <label class="field-label" for="w-outcome">What happened</label>
          <textarea
            id="w-outcome"
            v-model="record.outcome"
            class="input"
            rows="2"
            placeholder="Including anything that did not work — that is what the next reader needs"
          />
        </div>
        <div class="row">
          <button class="btn btn-primary" type="submit" :disabled="busy || !record.what_was_tried.trim()">
            Append to worklog
          </button>
          <button class="btn btn-secondary" type="button" @click="recording = false">Cancel</button>
          <span class="note">Append-only. This cannot be edited or removed afterwards.</span>
        </div>
      </form>
    </div>

    <!-- Actions live in the rail, not inline, so the reading order stays
         what happened -> what's next -> what I can do. -->
    <aside class="rail">
      <TransitionActions
        :from="detail.task.status"
        :moves="detail.can_move_to"
        :busy="busy"
        @move="startMove"
      />

      <form v-if="pending" class="moveform" @submit.prevent="move(pending)">
        <div v-for="(field, i) in fieldsFor(pending)" :key="field" class="field">
          <label class="field-label" :for="`f-${field}`">{{ PROMPTS[field] }}</label>
          <textarea
            :id="`f-${field}`"
            v-model="form[field]"
            class="input"
            rows="3"
            :autofocus="i === 0"
            :placeholder="PLACEHOLDERS[field]"
          />
        </div>
        <div class="row">
          <button class="btn btn-primary" type="submit" :disabled="!canSubmit || busy">
            {{ pending === 'blocked' ? 'Mark blocked' : 'Send back to active' }}
          </button>
          <button class="btn btn-secondary" type="button" @click="pending = null">Cancel</button>
        </div>
      </form>

      <section class="block rule-top">
        <span class="section-head">This task</span>
        <dl>
          <dt>reference</dt><dd class="mono">{{ detail.task.ref }}</dd>
          <dt>project</dt>
          <dd><RouterLink :to="`/p/${detail.task.project}`">{{ detail.task.project }}</RouterLink></dd>
          <dt>created</dt><dd><RelativeTime :value="detail.task.created_at" ago /></dd>
          <dt>worklog</dt>
          <dd>{{ detail.worklog.length }} {{ detail.worklog.length === 1 ? 'entry' : 'entries' }}</dd>
        </dl>
      </section>

      <section class="block rule-top">
        <button class="btn btn-ghost btn-block" @click="startEdit">Edit title &amp; body</button>
        <button class="btn btn-ghost btn-block btn-danger" @click="confirming = true">Delete task</button>
      </section>
    </aside>

    <ConfirmDialog
      :open="confirming"
      :title="`Delete ${detail.task.ref}?`"
      :confirm-word="detail.task.ref"
      :busy="busy"
      @cancel="confirming = false"
      @confirm="remove"
    >
      This removes the task, its state, and
      <strong>all {{ detail.worklog.length }} worklog
      {{ detail.worklog.length === 1 ? 'entry' : 'entries' }}</strong>.
      The worklog is append-only everywhere else in Cairn; this is the only way
      it is ever destroyed, and it cannot be undone.
    </ConfirmDialog>
  </div>
</template>

<style scoped>
.task { display: grid; grid-template-columns: minmax(0, 1fr) 310px; }
.pad { padding: var(--s-8); }

.content { padding: var(--s-8) var(--s-8) 40px; min-width: 0; }
.rail { padding: var(--s-8) var(--s-8) var(--s-8) 0; }

.crumbs { display: flex; gap: var(--s-2); font-size: var(--t-sm); margin-bottom: var(--s-4); }
.crumbs a { color: var(--text-dim); }

.statusline { display: flex; align-items: center; gap: var(--s-3); margin-bottom: var(--s-3); }
.since { font-size: var(--t-xs); color: var(--text-faint); }

h1 {
  font-size: var(--t-xl);
  font-weight: 500;
  letter-spacing: -0.01em;
  max-width: 32ch;
  margin-bottom: var(--s-4);
  text-wrap: pretty;
}

.body {
  font-size: var(--t-md);
  line-height: 1.6;
  color: #cfd3e5;
  max-width: 70ch;
  text-wrap: pretty;
  white-space: pre-wrap;
  margin-bottom: var(--s-8);
}

.edit { display: flex; flex-direction: column; gap: var(--s-3); margin-bottom: var(--s-8); }
.row { display: flex; gap: var(--s-3); align-items: center; }

/* Sits under the worklog and stays quiet until wanted: recording something is
   deliberate, and this is not the reason most visits happen. */
.record { margin-top: var(--s-4); padding-left: 0; }
.record-form { margin-top: var(--s-6); max-width: 620px; }
.record-form .field { margin-bottom: var(--s-4); }
.record-form .note { font-size: var(--t-sm); color: var(--text-faint); }

.moveform { display: flex; flex-direction: column; gap: var(--s-2); margin-top: var(--s-6); }
.moveform .field { margin-bottom: var(--s-2); }

.block { margin-top: var(--s-8); padding-top: var(--s-8); }
.block .section-head { display: block; margin-bottom: var(--s-4); }

dl { display: grid; grid-template-columns: auto 1fr; gap: var(--s-2) var(--s-6); font-size: 12.5px; }
dt { color: var(--text-dim); }
dd { color: var(--text-muted); }

@media (max-width: 1100px) {
  .task { grid-template-columns: minmax(0, 1fr); }
  .rail { padding: 0 var(--s-8) var(--s-8); }
}
</style>
