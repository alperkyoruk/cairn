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
    adopt(await api.login(username.value, password.value))
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

      <div class="field">
        <label for="username">username</label>
        <input id="username" v-model="username" autocomplete="username" autofocus />
      </div>

      <div class="field">
        <label for="password">password</label>
        <input id="password" v-model="password" type="password" autocomplete="current-password" />
      </div>

      <p v-if="failure" class="error">{{ failure }}</p>

      <button class="primary" type="submit" :disabled="busy">sign in</button>
    </form>
  </div>
</template>

<style scoped>
.gate { display: flex; justify-content: center; padding: 12vh var(--space-4); }
form { width: 22rem; }
h1 { margin: 0 0 var(--space-6); font-size: var(--text-xl); letter-spacing: -0.02em; }
button { width: 100%; }
</style>
