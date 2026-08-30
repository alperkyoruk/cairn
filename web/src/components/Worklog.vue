<script setup>
import { stamp } from '../time.js'
defineProps({ entries: Array })
</script>

<template>
  <section class="worklog">
    <h2>worklog</h2>
    <p v-if="!entries?.length" class="faint">Nothing recorded yet.</p>

    <ol v-else>
      <li v-for="entry in entries" :key="entry.id">
        <div class="meta faint mono">
          <span class="actor">{{ entry.actor }}</span>
          <span v-if="entry.to_status">
            {{ entry.from_status || 'new' }} → {{ entry.to_status }}
          </span>
          <span>{{ stamp(entry.created_at) }}</span>
        </div>
        <p v-if="entry.what_was_tried" class="tried">{{ entry.what_was_tried }}</p>
        <p v-if="entry.outcome" class="outcome muted">{{ entry.outcome }}</p>
      </li>
    </ol>
  </section>
</template>

<style scoped>
h2 {
  margin: 0 0 var(--space-3);
  font-size: var(--text-xs);
  font-family: var(--font-mono);
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--ink-muted);
}

ol {
  list-style: none;
  margin: 0;
  padding: 0 0 0 var(--space-4);
  border-left: 1px solid var(--border);
}

li { position: relative; padding-bottom: var(--space-4); }

li::before {
  content: '';
  position: absolute;
  left: calc(-1 * var(--space-4) - 3px);
  top: 0.55rem;
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--border-strong);
}

.meta {
  display: flex;
  gap: var(--space-3);
  font-size: var(--text-xs);
  flex-wrap: wrap;
}

.actor { color: var(--ink-muted); }

p { margin: var(--space-1) 0 0; white-space: pre-wrap; }
.outcome { font-size: var(--text-sm); }
</style>
