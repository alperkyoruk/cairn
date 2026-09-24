<script setup>
import { computed } from 'vue'
import ActorName from './ActorName.vue'
import RelativeTime from './RelativeTime.vue'
import { isSilent } from '../silence.js'

const props = defineProps({
  row: { type: Object, required: true },
  agents: { type: Array, default: () => [] },
  selected: { type: Boolean, default: false },
  dragging: { type: Boolean, default: false },
})
const emit = defineEmits(['open', 'toggle', 'grab'])

// The same three-way borrow the table does, kept identical on purpose: a
// blocked card shows the blocker because next_step is usually empty on one,
// and a card whose last event was an unaccompanied worklog entry shows what
// was recorded, because next_step still describes what the previous writer
// intended rather than what just happened.
const note = computed(() => {
  if (props.row.attempt) {
    return { kind: 'attempt', text: props.row.attempt.what_was_tried }
  }
  const state = props.row.state
  if (!state) return { kind: 'missing', text: 'no state yet' }
  if (props.row.task.status === 'blocked' && state.blocked_on) {
    return { kind: 'blocked', text: state.blocked_on }
  }
  if (!state.next_step) return { kind: 'empty', text: '' }
  return { kind: 'value', text: state.next_step }
})

const actor = computed(() => props.row.attempt?.actor ?? props.row.state?.updated_by)
const isAgent = computed(() => props.agents.some((a) => a.name === actor.value))
const silent = computed(() => isSilent(props.row.task))
</script>

<template>
  <article
    class="card"
    :class="{ selected, silent, dragging, waiting: row.task.status === 'review' }"
    @click="emit('open', $event)"
    @pointerdown="emit('grab', $event)"
  >
    <!-- The checkbox is the only part that selects. Clicking the card opens the
         task, which is what a card has always done, and quietly changing that
         to "selects" the moment a selection exists would make the board feel
         moded. -->
    <label class="pick" @click.stop>
      <input type="checkbox" :checked="selected" @change="emit('toggle', $event)" />
    </label>

    <div class="body">
      <div class="meta">
        <RouterLink :to="`/p/${row.task.project}`" class="project" @click.stop>
          {{ row.task.project }}
        </RouterLink>
        <span class="ref mono">{{ row.task.ref }}</span>
      </div>

      <p class="title">{{ row.task.title }}</p>

      <p v-if="note.text" class="note" :class="`note-${note.kind}`">{{ note.text }}</p>

      <div class="foot">
        <ActorName v-if="actor" :name="actor" :is-agent="isAgent" class="by" />
        <span v-else class="by faint">unclaimed</span>
        <RelativeTime class="ago" :value="row.task.updated_at" />
      </div>
    </div>
  </article>
</template>

<style scoped>
.card {
  position: relative;
  display: flex;
  gap: var(--s-2);
  padding: var(--s-3) var(--s-3) var(--s-3) var(--s-2);
  border-radius: var(--r-sm);
  background: var(--surface-raised);
  box-shadow: var(--e-1);
  cursor: pointer;
  transition: background var(--motion), box-shadow var(--motion);
}
.card:hover { background: var(--surface-high); }
/* The card does not move with the pointer -- a ghost follows it instead, so
   the column keeps its shape and you can still see where the card came from. */
.card.dragging { opacity: 0.35; }
.card.selected { box-shadow: 0 0 0 1px var(--accent); background: var(--accent-tint); }

/* Same edge language as the table: accent means it is waiting on you, amber
   means the work stopped and nobody said so. */
.card.waiting::before,
.card.silent::before {
  content: '';
  position: absolute;
  inset: var(--s-2) auto var(--s-2) 0;
  width: 2px;
  border-radius: 2px;
}
.card.waiting::before { background: var(--accent); }
.card.silent::before { background: var(--blocked); }

.pick { display: flex; align-items: flex-start; padding-top: 1px; }
.pick input { accent-color: var(--accent); cursor: pointer; margin: 0; }

.body { min-width: 0; flex: 1; display: flex; flex-direction: column; gap: var(--s-1); }

.meta { display: flex; align-items: baseline; gap: var(--s-2); }
.project { font-size: var(--t-xs); color: var(--text-dim); }
.project:hover { color: var(--accent); }
.ref { font-size: var(--t-xs); color: var(--text-faint); margin-left: auto; }

/* Two lines, then ellipsis. A card that grows to fit its title breaks the
   column rhythm, and the full text is one click away. */
.title {
  font-size: var(--t-base);
  color: var(--text);
  line-height: 1.35;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.note {
  font-size: var(--t-sm);
  color: var(--text-muted);
  line-height: 1.35;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.note-missing { color: var(--text-faint); }
.note-blocked { color: color-mix(in srgb, var(--blocked) 76%, var(--text-dim)); }
.note-attempt { color: color-mix(in srgb, var(--accent) 88%, var(--text-dim)); }

.foot {
  display: flex;
  align-items: baseline;
  gap: var(--s-2);
  margin-top: var(--s-1);
  font-size: var(--t-xs);
}
.by { color: var(--text-dim); min-width: 0; overflow: hidden; text-overflow: ellipsis; }
.by.faint { color: var(--text-faint); }
.ago { margin-left: auto; color: var(--text-dim); flex: none; }
.card.silent .ago { color: var(--blocked); }
</style>
