<script setup>
import { computed } from 'vue'
import ActorName from './ActorName.vue'
import RelativeTime from './RelativeTime.vue'

const props = defineProps({
  entry: { type: Object, required: true },
  agents: { type: Array, default: () => [] },
})

const moved = computed(() => Boolean(props.entry.to_status))
const intoBlocked = computed(() => props.entry.to_status === 'blocked')
const isAgent = computed(() => props.agents.some((a) => a.name === props.entry.actor))
</script>

<template>
  <li class="entry">
    <span class="marker" :class="{ moved, blocked: intoBlocked }" />

    <div class="body">
      <div class="meta">
        <ActorName :name="entry.actor" :is-agent="isAgent" class="actor" />

        <span v-if="moved" class="chip mono" :class="{ blocked: intoBlocked }">
          <span class="from">{{ entry.from_status || 'new' }}</span>
          <span class="arrow">→</span>
          <span class="to" :data-status="entry.to_status">{{ entry.to_status }}</span>
        </span>

        <RelativeTime class="at mono" :value="entry.created_at" absolute />
      </div>

      <p v-if="entry.what_was_tried" class="tried">{{ entry.what_was_tried }}</p>
      <!-- The luminance drop is the only thing separating outcome from attempt:
           no labels, no icons. -->
      <p v-if="entry.outcome" class="outcome">{{ entry.outcome }}</p>
    </div>
  </li>
</template>

<style scoped>
.entry {
  display: grid;
  grid-template-columns: 11px 1fr;
  gap: var(--s-6);
  padding-bottom: var(--s-8);
}

.marker {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  margin: 6px 0 0 3px;
  background: #595d6c;
}
.marker.moved { background: #968ae0; }
.marker.blocked {
  width: 7px;
  height: 7px;
  border-radius: 0;
  background: var(--blocked);
  transform: rotate(45deg);
  margin-left: 2px;
}

.meta {
  display: flex;
  align-items: center;
  gap: var(--s-3);
  flex-wrap: wrap;
}

.actor { font-size: 12.5px; color: #cfd3e5; }

.chip {
  font-size: var(--t-xs);
  padding: 2px 8px;
  border-radius: var(--r-sm);
  background: color-mix(in srgb, var(--text) 6%, transparent);
}
.chip.blocked { background: var(--blocked-tint); }
.chip .from { color: var(--text-dim); }
.chip .arrow { color: var(--text-dim); margin: 0 2px; }

.to[data-status='backlog'] { color: #9397ab; }
.to[data-status='queue']   { color: #cfd3e5; }
.to[data-status='active']  { color: #968ae0; }
.to[data-status='review']  { color: var(--accent); }
.to[data-status='done']    { color: #75798c; }
.to[data-status='blocked'] { color: var(--blocked); }

.at {
  margin-left: auto;
  font-size: var(--t-xs);
  color: var(--text-faint);
}

p { margin-top: var(--s-2); }
.tried { font-size: 13.5px; line-height: 1.55; color: #cfd3e5; max-width: 76ch; white-space: pre-wrap; }
.outcome { font-size: 13.5px; line-height: 1.55; color: #9397ab; max-width: 76ch; white-space: pre-wrap; }
</style>
