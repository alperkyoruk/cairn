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

// Moving to blocked is the one transition the server refuses without a value,
// so it is the one button that asks for something before it acts.
const blocking = ref(false)
const blockedOn = ref('')

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

async function move(to) {
  if (to === 'blocked' && !blocking.value) {
    blocking.value = true
    blockedOn.value = ''
    return
  }
  failure.value = ''
  busy.value = true
  try {
    // The status mark changing is the feedback; there is no toast.
    detail.value = await api.transition(
      props.taskRef, to,
      to === 'blocked' ? { where_i_left_off: '', next_step: '', blocked_on: blockedOn.value } : null,
      null,
    )
    blocking.value = false
  } catch (err) {
    failure.value = err.message
  } finally {
    busy.value = false
  }
}

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
    </div>

    <!-- Actions live in the rail, not inline, so the reading order stays
         what happened -> what's next -> what I can do. -->
    <aside class="rail">
      <TransitionActions
        :from="detail.task.status"
        :moves="detail.can_move_to"
        :busy="busy"
        @move="move"
      />

      <form v-if="blocking" class="blockform" @submit.prevent="move('blocked')">
        <label class="field-label" for="blockedon">What is blocking it</label>
        <textarea
          id="blockedon"
          v-model="blockedOn"
          class="input"
          rows="3"
          autofocus
          placeholder="Exactly what is needed in order to continue"
        />
        <div class="row">
          <button class="btn btn-primary" type="submit" :disabled="!blockedOn.trim() || busy">
            Mark blocked
          </button>
          <button class="btn btn-secondary" type="button" @click="blocking = false">Cancel</button>
        </div>
      </form>

      <section class="block rule-top">
        <span class="section-head">This task</span>
        <dl>
          <dt>reference</dt><dd class="mono">{{ detail.task.ref }}</dd>
          <dt>project</dt>
          <dd><RouterLink :to="`/p/${detail.task.project}`">{{ detail.task.project }}</RouterLink></dd>
          <dt>created</dt><dd><RelativeTime :value="detail.task.created_at" /> ago</dd>
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
.row { display: flex; gap: var(--s-3); }

.blockform { display: flex; flex-direction: column; gap: var(--s-2); margin-top: var(--s-6); }

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
