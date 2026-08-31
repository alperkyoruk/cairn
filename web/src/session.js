import { reactive } from 'vue'
import { api } from './api.js'

// Who is signed in, and whether anyone exists yet. Loaded once on boot; the
// answer decides between setup, login, and the app.
export const session = reactive({
  loading: true,
  needsSetup: false,
  needsSetupCode: false,
  actor: null,
})

export async function loadSession() {
  const data = await api.session()
  session.needsSetup = data.needs_setup
  session.needsSetupCode = Boolean(data.needs_setup_code)
  session.actor = data.actor
  session.loading = false
  return session
}

export function adopt(data) {
  session.needsSetup = data.needs_setup
  session.needsSetupCode = Boolean(data.needs_setup_code)
  session.actor = data.actor
}
