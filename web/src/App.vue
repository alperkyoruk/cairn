<script setup>
import { onMounted, ref } from 'vue'
import { session, loadSession } from './session.js'
import { api } from './api.js'
import Setup from './views/Setup.vue'
import Login from './views/Login.vue'

const failure = ref('')
const theme = ref(localStorage.getItem('cairn.theme') || 'system')

function applyTheme() {
  const root = document.documentElement
  if (theme.value === 'system') root.removeAttribute('data-theme')
  else root.setAttribute('data-theme', theme.value)
  localStorage.setItem('cairn.theme', theme.value)
}

function cycleTheme() {
  theme.value = { system: 'light', light: 'dark', dark: 'system' }[theme.value]
  applyTheme()
}

onMounted(async () => {
  applyTheme()
  try {
    await loadSession()
  } catch (err) {
    failure.value = err.message
    session.loading = false
  }
})

async function signOut() {
  await api.logout()
  session.actor = null
}
</script>

<template>
  <div v-if="session.loading" />

  <p v-else-if="failure" class="error boot">{{ failure }}</p>

  <Setup v-else-if="session.needsSetup" />
  <Login v-else-if="!session.actor" />

  <div v-else class="shell">
    <header>
      <nav class="row">
        <RouterLink to="/" class="wordmark">cairn</RouterLink>
        <RouterLink to="/agents" class="muted">agents</RouterLink>
      </nav>
      <div class="row">
        <span class="muted">{{ session.actor.name }}</span>
        <button @click="cycleTheme" :title="`theme: ${theme}`">
          {{ { system: '◐', light: '☀', dark: '☾' }[theme] }}
        </button>
        <button @click="signOut">sign out</button>
      </div>
    </header>
    <main>
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.boot { margin: var(--space-8); }

header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  padding: var(--space-3) var(--space-6);
  border-bottom: 1px solid var(--border);
  background: var(--bg-raised);
}

nav { gap: var(--space-4); }

.wordmark {
  font-weight: 600;
  letter-spacing: -0.01em;
}

main {
  max-width: 1100px;
  margin: 0 auto;
  padding: var(--space-6);
}
</style>
