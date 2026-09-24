<script setup>
import { onMounted, ref } from 'vue'
import { session, loadSession } from './session.js'
import Sidebar from './components/Sidebar.vue'
import SetupView from './views/SetupView.vue'
import LoginView from './views/LoginView.vue'

const failure = ref('')

onMounted(async () => {
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

  <div v-else class="shell">
    <Sidebar :actor="session.actor" />
    <main><RouterView /></main>
  </div>
</template>

<style scoped>
.boot { margin: var(--s-12); }

/* A rail and the rest. The board wants every pixel of width it can get -- five
   columns divided by a 1280px measure is five narrow slivers on a monitor that
   had the room -- so the cap is gone and the rail holds the left edge instead.
   Pages that are still text rather than columns set their own measure. */
.shell {
  display: grid;
  grid-template-columns: 200px minmax(0, 1fr);
  min-height: 100dvh;
}
main { min-width: 0; }

@media (max-width: 860px) {
  .shell { grid-template-columns: minmax(0, 1fr); }
}
</style>
