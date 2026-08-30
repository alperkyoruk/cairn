<script setup>
import { ref } from 'vue'
import { api } from '../api.js'
import { adopt } from '../session.js'

const username = ref('')
const password = ref('')
const failure = ref('')
const busy = ref(false)

async function submit() {
  failure.value = ''
  busy.value = true
  try {
    adopt(await api.setup(username.value, password.value))
  } catch (err) {
    failure.value = err.message
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="gate">
    <form class="card" @submit.prevent="submit">
      <h1>cairn</h1>
      <p class="muted intro">
        This is a fresh install. Pick a name and a password — they are stored on
        this machine and nowhere else.
      </p>

      <div class="field">
        <label for="username">username</label>
        <input id="username" v-model="username" autocomplete="username" autofocus />
      </div>

      <div class="field">
        <label for="password">password</label>
        <input id="password" v-model="password" type="password" autocomplete="new-password" />
      </div>

      <p v-if="failure" class="error">{{ failure }}</p>

      <button class="primary" type="submit" :disabled="busy">start</button>

      <p class="faint note">
        There is no email and no reset link. If you lose this password, the only
        way back in is <code>cairn --reset-password</code> on this machine.
      </p>
    </form>
  </div>
</template>

<style scoped>
.gate { display: flex; justify-content: center; padding: 12vh var(--space-4); }
form { width: 22rem; }
h1 { margin: 0; font-size: var(--text-xl); letter-spacing: -0.02em; }
.intro { font-size: var(--text-sm); margin: var(--space-2) 0 var(--space-6); }
button { width: 100%; }
.note { font-size: var(--text-xs); margin: var(--space-4) 0 0; }
code { font-family: var(--font-mono); }
</style>
