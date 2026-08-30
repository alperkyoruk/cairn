<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api.js'
import WarningTriangle from '../components/icons/WarningTriangle.vue'
import RelativeTime from '../components/RelativeTime.vue'

const agents = ref([])
const tokens = ref([])
const failure = ref('')
const name = ref('')
const busy = ref(false)

// The plaintext token lives only here, in the component that received it.
// It is never persisted and never put in a store that survives navigation.
const fresh = ref(null)
const copied = ref(false)

async function load() {
  const list = await api.agents()
  agents.value = list
  const rows = await Promise.all(list.map(async (agent) => {
    const issued = await api.tokens(agent.id)
    return issued.map((token) => ({ ...token, agent: agent.name }))
  }))
  tokens.value = rows.flat()
}

async function create() {
  failure.value = ''
  busy.value = true
  try {
    const created = await api.createAgent(name.value)
    fresh.value = { agent: created.agent.name, token: created.token }
    copied.value = false
    name.value = ''
    await load()
  } catch (err) {
    failure.value = err.message
  } finally {
    busy.value = false
  }
}

async function copy() {
  try {
    await navigator.clipboard.writeText(fresh.value.token)
    copied.value = true
    setTimeout(() => { copied.value = false }, 1500)
  } catch {
    failure.value = 'Could not reach the clipboard — select the token and copy it by hand.'
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

// Only a hash of the secret is stored, plus its opening characters. That is
// enough to match a row here against the token sitting in an agent's config,
// which is the question this column exists to answer -- and the tail stays
// unrecoverable, which is the point of hashing it.
function elide(token) {
  return token.prefix ? `${token.prefix}…` : token.name
}

onMounted(load)
</script>

<template>
  <div class="agents">
    <h1>Agents</h1>

    <p v-if="failure" class="error">{{ failure }}</p>

    <div class="grid">
      <div class="list">
        <!-- The one place in the app that raises its voice, because it is the
             only screen where dismissing it loses something unrecoverable. -->
        <div v-if="fresh" class="reveal">
          <p class="shout">
            <WarningTriangle />
            Copy this now — it is shown once
          </p>

          <div class="strip">
            <code class="token mono">{{ fresh.token }}</code>
            <button class="btn btn-primary" @click="copy">{{ copied ? 'Copied' : 'Copy' }}</button>
          </div>

          <p class="explain">
            Cairn stores only a hash of it. Close this panel without copying and the
            token is gone — the only way forward is to <strong>issue a new one</strong>
            and revoke this. Paste it into <code class="mono">{{ fresh.agent }}</code>'s
            MCP config as the bearer token for
            <code class="mono">http://localhost:7777/mcp</code>.
          </p>

          <button class="btn btn-ghost" @click="fresh = null">Done</button>
        </div>

        <p v-if="!tokens.length" class="empty">
          No agents yet. Register one to give it a token.
        </p>

        <table v-else>
          <thead>
            <tr>
              <th class="c-agent">Agent</th>
              <th class="c-token">Token</th>
              <th class="c-used">Last used</th>
              <th class="c-act"></th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="token in tokens"
              :key="token.id"
              class="rule-bottom"
              :class="{ unused: !token.last_used_at, revoked: token.revoked_at }"
            >
              <td class="c-agent">{{ token.agent }}</td>
              <td class="c-token">
                <span class="mono">{{ elide(token) }}</span>
                <span class="tname">{{ token.name }}</span>
              </td>
              <td class="c-used">
                <RelativeTime v-if="token.last_used_at" :value="token.last_used_at" />
                <span v-else class="faint">never</span>
              </td>
              <td class="c-act">
                <button v-if="!token.revoked_at" class="btn btn-ghost" @click="revoke(token.id)">
                  Revoke
                </button>
                <span v-else class="faint">revoked</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <aside class="register">
        <span class="section-head">Register an agent</span>
        <form @submit.prevent="create">
          <label class="field-label" for="agentname">Name</label>
          <input id="agentname" v-model="name" class="input input-mono" placeholder="claude" />
          <button class="btn btn-primary btn-block create" type="submit" :disabled="!name || busy">
            Create token
          </button>
        </form>
        <p class="note">
          The name is what appears in <code class="mono">last updated by</code> and on
          every worklog entry that agent writes. Pick the one you will recognise a
          month from now.
        </p>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.agents { padding: var(--s-6); }
h1 { font-size: var(--t-xl); font-weight: 500; letter-spacing: -0.01em; margin-bottom: var(--s-8); }

.grid { display: grid; grid-template-columns: minmax(0, 1fr) 340px; gap: var(--s-12); }

.reveal {
  border-radius: var(--r-lg);
  background: color-mix(in srgb, var(--accent) 7%, var(--surface-raised));
  box-shadow: inset 0 0 0 1px var(--accent);
  padding: var(--s-6);
  margin-bottom: var(--s-8);
}

.shout {
  display: flex;
  align-items: center;
  gap: var(--s-2);
  color: var(--accent);
  font-size: var(--t-xs);
  text-transform: uppercase;
  letter-spacing: 0.08em;
  margin-bottom: var(--s-4);
}

.strip {
  display: flex;
  align-items: center;
  gap: var(--s-4);
  background: var(--bg);
  border-radius: var(--r-md);
  padding: var(--s-3) var(--s-4);
}
.token { font-size: 16px; color: var(--text); word-break: break-all; flex: 1; }

.explain {
  font-size: var(--t-base);
  line-height: 1.6;
  color: #cfd3e5;
  margin: var(--s-4) 0;
  max-width: 68ch;
  text-wrap: pretty;
}

table { width: 100%; table-layout: fixed; border-collapse: collapse; }
th {
  text-align: left;
  font-weight: 400;
  font-size: var(--t-xs);
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: color-mix(in srgb, var(--text) 60%, transparent);
  padding: 0 var(--s-2) var(--s-2);
}
td { padding: var(--s-3) var(--s-2); font-size: 12.5px; vertical-align: middle; }

.c-agent { width: 150px; }
.c-token { color: var(--text-dim); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tname { margin-left: var(--s-3); color: var(--text-faint); }
.c-used { width: 100px; }
.c-act { width: 90px; text-align: right; }

.unused { color: var(--text-dim); }
.revoked { opacity: 0.45; }

.register form { margin: var(--s-4) 0; }
.create { justify-content: center; margin-top: var(--s-3); }
.note { font-size: var(--t-sm); color: var(--text-faint); line-height: 1.6; text-wrap: pretty; }

.empty { color: var(--text-faint); padding: var(--s-8) 0; }

@media (max-width: 1100px) {
  .grid { grid-template-columns: minmax(0, 1fr); gap: var(--s-8); }
}
</style>
