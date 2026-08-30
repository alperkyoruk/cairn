<script setup>
import CairnMark from './icons/CairnMark.vue'

defineProps({ actor: Object })
</script>

<template>
  <nav class="nav rule-bottom">
    <div class="inner">
      <RouterLink to="/" class="brand">
        <CairnMark :size="15" class="mark" />
        <span class="wordmark">cairn</span>
      </RouterLink>

      <RouterLink to="/" class="link">Board</RouterLink>
      <RouterLink to="/agents" class="link">Agents</RouterLink>

      <!-- A label, not a menu. There is nothing behind it. -->
      <span class="who">{{ actor?.name }}</span>
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
</style>
