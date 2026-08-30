<script setup>
import { computed, ref } from 'vue'
import WorklogEntry from './WorklogEntry.vue'
import ChevronDown from './icons/ChevronDown.vue'
import { formatRelative } from '../composables/useRelativeTime.js'

// A rail, not a list. No card, no enclosure -- the geometry is what makes this
// read as a history rather than a record.
const props = defineProps({
  entries: { type: Array, default: () => [] },
  agents: { type: Array, default: () => [] },
})

const KEEP_EXPANDED = 5
const expanded = ref(false)

const older = computed(() =>
  props.entries.length > KEEP_EXPANDED ? props.entries.slice(0, -KEEP_EXPANDED) : [])
const recent = computed(() =>
  props.entries.length > KEEP_EXPANDED ? props.entries.slice(-KEEP_EXPANDED) : props.entries)

// The collapse sits at the top, which is where the old entries are given the
// oldest-to-newest ordering, so the recent ones are never the ones hidden.
const olderRange = computed(() => {
  if (!older.value.length) return ''
  const day = (v) => new Date(v).toLocaleDateString('en-GB', { day: 'numeric', month: 'short' })
  const first = day(older.value[0].created_at)
  const last = day(older.value[older.value.length - 1].created_at)
  return first === last ? first : `${first} – ${last}`
})
</script>

<template>
  <section>
    <header>
      <h2>Worklog</h2>
      <span class="legend mono">
        {{ entries.length }} {{ entries.length === 1 ? 'entry' : 'entries' }} · append-only
      </span>
    </header>

    <p v-if="!entries.length" class="faint">Nothing recorded yet.</p>

    <ol v-else class="rail">
      <li v-if="older.length && !expanded" class="collapsed">
        <button class="reveal mono" @click="expanded = true">
          <ChevronDown />
          {{ older.length }} earlier {{ older.length === 1 ? 'entry' : 'entries' }} · {{ olderRange }}
        </button>
      </li>

      <template v-if="expanded">
        <WorklogEntry v-for="e in older" :key="e.id" :entry="e" :agents="agents" />
      </template>

      <WorklogEntry v-for="e in recent" :key="e.id" :entry="e" :agents="agents" />
    </ol>
  </section>
</template>

<style scoped>
header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--s-4);
  margin-bottom: var(--s-6);
}

h2 { font-size: var(--t-lg); font-weight: 500; }
.legend { font-size: 11.5px; color: var(--text-faint); }

.rail {
  padding-left: var(--s-3);
  background: linear-gradient(to bottom,
    transparent, var(--rule) 20px,
    var(--rule) calc(100% - 20px), transparent) no-repeat 6px 0 / 1px 100%;
}

.collapsed { padding-bottom: var(--s-8); margin-left: -2px; }

.reveal {
  display: inline-flex;
  align-items: center;
  gap: var(--s-2);
  font: inherit;
  font-size: var(--t-sm);
  color: var(--text-dim);
  background: var(--bg);
  border: 0;
  padding: 0 var(--s-2) 0 0;
  cursor: pointer;
}
.reveal:hover { color: var(--accent); }
</style>
