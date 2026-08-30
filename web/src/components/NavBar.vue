<script setup>
import CairnMark from './icons/CairnMark.vue'
import ThemeMoon from './icons/ThemeMoon.vue'
import { useTheme } from '../composables/useTheme.js'

defineProps({ actor: Object })
const { theme, cycle } = useTheme()
</script>

<template>
  <nav class="nav rule-bottom">
    <RouterLink to="/" class="brand">
      <CairnMark :size="15" class="mark" />
      <span class="wordmark">cairn</span>
    </RouterLink>

    <RouterLink to="/" class="link">Board</RouterLink>
    <RouterLink to="/agents" class="link">Agents</RouterLink>

    <button class="btn btn-icon" :title="`Theme: ${theme}`" @click="cycle">
      <ThemeMoon />
    </button>

    <!-- A label, not a menu. There is nothing behind it. -->
    <span class="who">{{ actor?.name }}</span>
  </nav>
</template>

<style scoped>
.nav {
  display: flex;
  align-items: center;
  gap: var(--s-4);
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

.who { font-size: 12.5px; color: var(--text-dim); }
</style>
