<script setup>
import CairnMark from './icons/CairnMark.vue'
import { api } from '../api.js'
import { session } from '../session.js'

defineProps({ actor: Object })

async function signOut() {
  // Revokes the session token server-side, not just the cookie, so a copied
  // cookie is dead too.
  try {
    await api.logout()
  } finally {
    session.actor = null
  }
}
</script>

<template>
  <nav class="nav rule-bottom">
    <div class="inner">
      <RouterLink to="/" class="brand">
        <CairnMark :size="15" class="mark" />
        <span class="wordmark">cairn</span>
      </RouterLink>

      <RouterLink to="/" class="link">Projects</RouterLink>
      <RouterLink to="/tasks" class="link">Tasks</RouterLink>
      <RouterLink to="/agents" class="link">Agents</RouterLink>

      <!-- The design has the username as a label with nothing behind it, which
           left no way to sign out at all. Rather than turn it into a menu, the
           way out sits beside it and stays quiet. -->
      <span class="who">{{ actor?.name }}</span>
      <button class="signout" @click="signOut">sign out</button>
    </div>
  </nav>
</template>

<style scoped>
.nav { padding: 0; }

/* Same measure as <main>, so the wordmark sits over the table's left edge and
   the username sits over its right one. */
.inner {
  display: flex;
  align-items: center;
  gap: var(--s-4);
  max-width: 1280px;
  margin: 0 auto;
  padding: var(--s-3) var(--s-6);
}

.brand {
  display: inline-flex;
  align-items: center;
  gap: var(--s-2);
  margin-right: auto;
}
.brand:hover { color: inherit; }
.mark { color: var(--accent); }
.wordmark { font-size: 18px; letter-spacing: -0.01em; }

.link { font-size: 14px; }
.link:hover { color: var(--accent); }
.link.router-link-exact-active { color: var(--accent); }

.who { font-size: 12.5px; color: var(--text-dim); margin-left: var(--s-2); }

.signout {
  font: inherit;
  font-size: 12.5px;
  color: var(--text-faint);
  background: none;
  border: 0;
  padding: 0;
  cursor: pointer;
  transition: color var(--motion);
}
.signout:hover { color: var(--text-muted); }
</style>
