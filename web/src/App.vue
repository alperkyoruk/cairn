<script setup>
import { onMounted, ref } from 'vue'
import { session, loadSession } from './session.js'
import { useTheme } from './composables/useTheme.js'
import NavBar from './components/NavBar.vue'
import SetupView from './views/SetupView.vue'
import LoginView from './views/LoginView.vue'

const failure = ref('')
const { apply } = useTheme()

onMounted(async () => {
  apply()
  try {
    await loadSession()
  } catch (err) {
    failure.value = err.message
    session.loading = false
  }
})
</script>

<template>
  <!-- The board is a local SQLite read behind a same-origin call; it is fast.
       Render nothing rather than a skeleton, and speak only if it fails. -->
  <div v-if="session.loading" />

  <p v-else-if="failure" class="error boot">{{ failure }}</p>

  <SetupView v-else-if="session.needsSetup" />
  <LoginView v-else-if="!session.actor" />

  <template v-else>
    <NavBar :actor="session.actor" />
    <main><RouterView /></main>
  </template>
</template>

<style scoped>
.boot { margin: var(--s-12); }
main { max-width: 1280px; }
</style>
