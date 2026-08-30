<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api.js'
import { ago, stamp } from '../time.js'

const agents = ref([])
const tokens = ref({})
const failure = ref('')
const name = ref('')
// Shown exactly once, then gone forever -- the server keeps only a hash.
const freshToken = ref(null)

async function load() {
  agents.value = await api.agents()
  for (const agent of agents.value) {
    tokens.value[agent.id] = await api.tokens(agent.id)
  }
}

async function create() {
  failure.value = ''
  try {
    const created = await api.createAgent(name.value)
    freshToken.value = { agent: created.agent.name, token: created.token }
    name.value = ''
    await load()
  } catch (err) {
    failure.value = err.message
  }
}

async function issue(agent) {
  failure.value = ''
  try {
    const { token } = await api.issueToken(agent.id, 'replacement token')
    freshToken.value = { agent: agent.name, token }
    await load()
  } catch (err) {
    failure.value = err.message
  }
}

async function revoke(id) {
  failure.value = ''
  try {
    await api.revokeToken(id)
    await load()
  } catch (err) {
    failure.value = err.message
  }
}

onMounted(load)
</script>

<template>
  <div>
    <h1>agents</h1>
    <p class="muted intro">
      Each agent gets a token to put in its MCP configuration. Cairn stores only
      a hash of it, so a token is visible once and never again — if one is lost,
      issue another and revoke the old one.
    </p>

    <p v-if="failure" class="error">{{ failure }}</p>

    <div v-if="freshToken" class="card fresh">
      <p class="key">token for {{ freshToken.agent }} — copy it now</p>
      <code class="mono">{{ freshToken.token }}</code>
      <button @click="freshToken = null">done</button>
    </div>

    <form class="row add" @submit.prevent="create">
      <input v-model="name" placeholder="agent name (claude, codex)" />
      <button class="primary" type="submit" :disabled="!name">add agent</button>
    </form>

    <p v-if="!agents.length" class="empty">No agents yet.</p>

    <div v-for="agent in agents" :key="agent.id" class="card agent">
      <div class="spread">
        <strong>{{ agent.name }}</strong>
        <button @click="issue(agent)">issue token</button>
      </div>
      <table>
        <tbody>
          <tr v-for="token in tokens[agent.id]" :key="token.id" :class="{ dead: token.revoked_at }">
            <td>{{ token.name }}</td>
            <td class="faint">
              <span v-if="token.revoked_at">revoked</span>
              <span v-else-if="token.last_used_at" :title="stamp(token.last_used_at)">
                used {{ ago(token.last_used_at) }} ago
              </span>
              <span v-else>never used</span>
            </td>
            <td class="right">
              <button v-if="!token.revoked_at" class="danger" @click="revoke(token.id)">revoke</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
h1 { margin: 0; font-size: var(--text-xl); letter-spacing: -0.02em; }
.intro { font-size: var(--text-sm); max-width: 46rem; margin: var(--space-2) 0 var(--space-6); }

.fresh { border-color: var(--accent); margin-bottom: var(--space-4); }
.fresh .key { margin: 0 0 var(--space-2); font-size: var(--text-sm); color: var(--ink-muted); }
.fresh code {
  display: block;
  padding: var(--space-3);
  background: var(--bg-sunken);
  border-radius: var(--radius-sm);
  word-break: break-all;
  margin-bottom: var(--space-3);
}

.add { margin-bottom: var(--space-6); }
.add input { max-width: 22rem; }

.agent { margin-bottom: var(--space-3); }
table { width: 100%; border-collapse: collapse; font-size: var(--text-sm); margin-top: var(--space-3); }
td { padding: var(--space-2) 0; border-top: 1px solid var(--border); }
.right { text-align: right; }
.dead { opacity: 0.5; }
</style>
