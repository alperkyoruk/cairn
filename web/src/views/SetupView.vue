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
    adopt(await api.setup(username.value, password.value))
  } catch (err) {
    failure.value = err.message
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <!-- A person naming themselves on their own server, not creating an account. -->
  <div class="gate">
    <div class="panel-plain">
      <CairnMark :size="20" class="mark" />
      <h1>Name yourself</h1>
      <p class="lead">
        This is your server. The name you pick here is the one that appears next
        to every task you touch.
      </p>

      <form @submit.prevent="submit">
        <label class="field-label" for="username">Username</label>
        <input id="username" v-model="username" class="input" autocomplete="username" autofocus />

        <label class="field-label sp" for="password">Password</label>
        <input id="password" v-model="password" class="input" type="password" autocomplete="new-password" />

        <p v-if="failure" class="error">{{ failure }}</p>

        <button class="btn btn-primary start" type="submit" :disabled="busy">Start</button>
      </form>

      <p class="warn">
        There is no reset email and never will be. If you forget this password,
        stop the server and run <code class="mono">cairn --reset-password</code>
        on the machine it runs on.
      </p>
    </div>
  </div>
</template>

<style scoped>
.gate {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  padding: 64px 56px;
}
.panel-plain { width: 100%; max-width: 340px; }
.mark { color: var(--accent); margin-bottom: var(--s-6); display: block; }

h1 { font-size: 22px; font-weight: 500; letter-spacing: -0.01em; }
.lead {
  font-size: var(--t-base);
  color: var(--text-muted);
  line-height: 1.6;
  margin: var(--s-3) 0 var(--s-8);
  text-wrap: pretty;
}
.sp { margin-top: var(--s-4); }
.start { margin-top: var(--s-6); }

.warn {
  font-size: 12.5px;
  color: var(--text-dim);
  line-height: 1.6;
  margin-top: var(--s-8);
  text-wrap: pretty;
}
</style>
