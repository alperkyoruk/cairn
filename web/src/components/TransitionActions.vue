<script setup>
import { computed } from 'vue'

// The buttons are whatever the server said is legal, in the order it said it.
// GET /api/tasks/{ref} returns can_move_to for the signed-in actor, ordered so
// the move that carries the work forward comes first. Never a disabled grid of
// every status, and never a legal-move list derived on the client.
const props = defineProps({
  from: { type: String, required: true },
  moves: { type: Array, default: () => [] },
  busy: Boolean,
})
defineEmits(['move'])

// Labels are per (from, to), because "active" means "start work" from queue and
// "send it back" from review.
const LABELS = {
  'backlog>queue': 'Queue it',
  'queue>active': 'Start work',
  'queue>backlog': 'Back to backlog',
  'active>review': 'Send to review',
  'active>blocked': 'Mark blocked',
  'review>done': 'Mark done',
  'review>active': 'Send back to active',
  'blocked>active': 'Unblock — back to active',
  'blocked>backlog': 'Send to backlog',
  'done>queue': 'Reopen to queue',
  'done>active': 'Reopen to active',
}

const buttons = computed(() =>
  props.moves.map((to) => ({
    to,
    label: LABELS[`${props.from}>${to}`] ?? `Move to ${to}`,
  })))
</script>

<template>
  <div class="actions">
    <span class="section-head">Available now</span>

    <p v-if="!buttons.length" class="none">
      Nothing to do here from <strong>{{ from }}</strong>.
    </p>

    <button
      v-for="(button, i) in buttons"
      :key="button.to"
      class="btn btn-block"
      :class="i === 0 ? 'btn-primary' : 'btn-secondary'"
      :disabled="busy"
      @click="$emit('move', button.to)"
    >
      {{ button.label }}
    </button>
  </div>
</template>

<style scoped>
.actions { display: flex; flex-direction: column; gap: var(--s-2); }
.section-head { margin-bottom: var(--s-2); }
.none { font-size: var(--t-sm); color: var(--text-faint); }
</style>
