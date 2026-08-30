<script setup>
import { ago, stamp } from '../time.js'
defineProps({ state: Object })
</script>

<template>
  <section class="state card">
    <h2>state</h2>

    <p v-if="!state" class="empty">
      Nobody has left a note on this task yet.
    </p>

    <template v-else>
      <div v-if="state.blocked_on" class="blocked">
        <span class="key">blocked on</span>
        <p>{{ state.blocked_on }}</p>
      </div>

      <div class="field-row">
        <span class="key">where I left off</span>
        <p>{{ state.where_i_left_off || '—' }}</p>
      </div>

      <div class="field-row">
        <span class="key">next step</span>
        <p class="next">{{ state.next_step || '—' }}</p>
      </div>

      <p class="byline faint">
        written by {{ state.updated_by }} · <span :title="stamp(state.updated_at)">{{ ago(state.updated_at) }} ago</span>
      </p>
    </template>
  </section>
</template>

<style scoped>
/* The state block is one panel, not a list of entries, because there is only
   ever one of it. The worklog below is what a list looks like. */
.state { border-left: 3px solid var(--accent); }

h2 {
  margin: 0 0 var(--space-3);
  font-size: var(--text-xs);
  font-family: var(--font-mono);
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--ink-muted);
}

.field-row, .blocked { margin-bottom: var(--space-3); }

.key {
  display: block;
  font-size: var(--text-xs);
  color: var(--ink-faint);
  margin-bottom: 2px;
}

p { margin: 0; white-space: pre-wrap; }

.next { font-weight: 500; }

.blocked {
  border-left: 2px solid var(--status-blocked);
  padding-left: var(--space-3);
  margin-left: calc(-1 * var(--space-1));
}
.blocked .key { color: var(--status-blocked); }

.byline { font-size: var(--text-xs); margin-top: var(--space-4); }
.empty { padding: var(--space-4) 0; text-align: left; }
</style>
