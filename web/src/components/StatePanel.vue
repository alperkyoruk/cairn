<script setup>
import { computed } from 'vue'
import ActorName from './ActorName.vue'
import RelativeTime from './RelativeTime.vue'

// A panel reads as a record; the worklog's rail reads as a history. That
// opposite geometry is what stops state from being misread as the most recent
// comment -- so this stays enclosed, and the worklog never gets a card.
const props = defineProps({
  state: Object,
  status: String,
  agents: { type: Array, default: () => [] },
})

const blocked = computed(() => props.status === 'blocked' && props.state?.blocked_on)

const markColor = computed(() => {
  if (props.status === 'blocked') return 'var(--blocked)'
  if (props.status === 'active' || props.status === 'review') return 'var(--accent)'
  return 'transparent'
})

const isAgent = computed(() => props.agents.some((a) => a.name === props.state?.updated_by))
</script>

<template>
  <section class="state" :style="{ '--mark': markColor }">
    <header>
      <span class="section-head">State</span>
      <!-- This legend is doing real work: it is what tells a first-time reader
           that this is not a comment. -->
      <span class="legend mono">one record · overwritten in place</span>
    </header>

    <p v-if="!state" class="absent">
      No state yet. The first agent to work this task writes one; until then the
      body is all there is.
    </p>

    <template v-else>
      <!-- When the task is blocked this is the most important text on the
           screen, so it comes first and is the largest value in the panel. -->
      <div v-if="blocked" class="blocked-box">
        <span class="key mono">blocked_on</span>
        <p class="blocked-value">{{ state.blocked_on }}</p>
      </div>

      <div class="field">
        <span class="key mono">where_i_left_off</span>
        <p class="value">{{ state.where_i_left_off || '—' }}</p>
      </div>

      <div class="field">
        <span class="key mono">next_step</span>
        <p class="value">{{ state.next_step || '—' }}</p>
      </div>

      <p class="byline">
        written by <ActorName :name="state.updated_by" :is-agent="isAgent" /> ·
        <RelativeTime :value="state.updated_at" ago />
      </p>
    </template>
  </section>
</template>

<style scoped>
.state {
  background: var(--surface-raised);
  border-radius: var(--r-lg);
  padding: var(--s-6) var(--s-8) var(--s-8);
  box-shadow: var(--e-1), inset 2px 0 0 var(--mark);
  margin-bottom: var(--s-12);
}

header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--s-4);
  margin-bottom: var(--s-6);
}

.legend { font-size: 11.5px; color: var(--text-faint); }

.field { margin-bottom: var(--s-6); }

/* Mono keys say "these are three fields of one database row", not "three
   sentences somebody wrote". */
.key {
  display: block;
  font-size: var(--t-xs);
  letter-spacing: 0.06em;
  color: var(--text-dim);
  margin-bottom: var(--s-2);
}

.value {
  font-size: var(--t-md);
  line-height: 1.55;
  color: #e4e7f5;
  max-width: 70ch;
  text-wrap: pretty;
  white-space: pre-wrap;
}

.blocked-box {
  padding: var(--s-4) var(--s-6);
  border-radius: var(--r-md);
  background: var(--blocked-tint);
  margin-bottom: var(--s-6);
}
.blocked-box .key { color: var(--blocked); }
.blocked-value {
  font-size: 16px;
  line-height: 1.55;
  color: var(--text);
  max-width: 70ch;
  text-wrap: pretty;
  white-space: pre-wrap;
}

.byline {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: var(--t-sm);
  color: var(--text-faint);
}

.absent {
  font-size: var(--t-md);
  line-height: 1.55;
  color: var(--text-faint);
  max-width: 60ch;
  text-wrap: pretty;
}
</style>
