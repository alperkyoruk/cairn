<script setup>
import { computed, onMounted, ref } from 'vue'
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
    return issued.map((token) => ({ ...token, agent: agent.name, agentId: agent.id }))
  }))
  tokens.value = rows.flat()
}

// Grouped by agent, not by token. A flat token list repeats the name once per
// token, so an agent holding two reads as two agents with the same name -- and
// there was nowhere to hang a per-agent action, which is why the only way to
// get a token was to register somebody new.
const byAgent = computed(() =>
  agents.value.map((agent) => ({
    ...agent,
    tokens: tokens.value.filter((t) => t.agentId === agent.id),
  })))

// Issuing to an existing agent rather than creating one.
//
// Revoking used to be a one-way door: the register form is the only thing that
// hands back a secret, and a duplicate name is refused, so revoking claude's
// leaked token left the choice of inventing claude-2 -- orphaning the identity
// on every worklog entry claude had ever written -- or calling the API by hand.
async function issue(agent) {
  failure.value = ''
  busy.value = true
  try {
    const issued = await api.issueToken(agent.id, 'issued from the interface')
    fresh.value = { agent: agent.name, token: issued.token }
    copied.value = false
    await load()
  } catch (err) {
    failure.value = err.message
  } finally {
    busy.value = false
  }
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

        <p v-if="!agents.length" class="empty">
          No agents yet. Register one to give it a token.
        </p>

        <!-- One block per agent, because the agent is the thing that persists:
             tokens come and go under it, and the identity on every worklog entry
             it has ever written stays. -->
        <section v-for="agent in byAgent" :key="agent.id" class="agent">
          <div class="agent-head rule-bottom">
            <span class="agent-name">{{ agent.name }}</span>
            <button class="btn btn-ghost" :disabled="busy" @click="issue(agent)">
              Issue token
            </button>
          </div>

          <p v-if="!agent.tokens.length" class="faint none">
            No tokens. Issue one to let it connect.
          </p>

          <table v-else>
            <tbody>
              <tr
                v-for="token in agent.tokens"
                :key="token.id"
                class="rule-bottom"
                :class="{ unused: !token.last_used_at, revoked: token.revoked_at }"
              >
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
        </section>
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

/* The agent is the heading now, so the name is no longer a repeated column. */
.agent { margin-bottom: var(--s-8); }
.agent-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--s-4);
  padding-bottom: var(--s-3);
}
.agent-name { font-size: var(--t-md); }
.none { font-size: 12.5px; padding: var(--s-3) 0; }

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
