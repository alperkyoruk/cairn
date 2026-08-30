<script setup>
import { ref } from 'vue'
import { api } from '../api.js'
import { adopt } from '../session.js'
import CairnMark from '../components/icons/CairnMark.vue'

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
    <div class="panel-plain">
      <CairnMark :size="20" class="mark" />
      <h1>cairn</h1>

      <form @submit.prevent="submit">
        <label class="field-label" for="username">Username</label>
        <input id="username" v-model="username" class="input" autocomplete="username" autofocus />

        <label class="field-label sp" for="password">Password</label>
        <input id="password" v-model="password" class="input" type="password" autocomplete="current-password" />

        <p v-if="failure" class="error">{{ failure }}</p>

        <button class="btn btn-primary start" type="submit" :disabled="busy">Sign in</button>
      </form>
    </div>
  </div>
</template>

<style scoped>
.gate { display: flex; padding: 64px 56px; }
.panel-plain { min-height: 340px; max-width: 300px; }
.mark { color: var(--accent); margin-bottom: var(--s-6); display: block; }
h1 { font-size: 22px; font-weight: 500; letter-spacing: -0.01em; margin-bottom: var(--s-8); }
.sp { margin-top: var(--s-4); }
.start { margin-top: var(--s-6); }
</style>
