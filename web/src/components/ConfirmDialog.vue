<script setup>
import { ref, watch } from 'vue'

// Type-to-confirm, because this is the one place in Cairn the append-only
// worklog is ever destroyed.
const props = defineProps({
  open: Boolean,
  title: String,
  body: String,
  confirmWord: String,
  confirmLabel: { type: String, default: 'Delete permanently' },
  busy: Boolean,
})
const emit = defineEmits(['confirm', 'cancel'])

const typed = ref('')
watch(() => props.open, () => { typed.value = '' })
</script>

<template>
  <div v-if="open" class="scrim" @click.self="$emit('cancel')">
    <div class="dialog" role="dialog" aria-modal="true">
      <h2>{{ title }}</h2>
      <p class="body"><slot>{{ body }}</slot></p>

      <label class="prompt">
        Type <code class="mono">{{ confirmWord }}</code> to confirm.
      </label>
      <input v-model="typed" class="input input-mono" autofocus />

      <div class="row">
        <button class="btn btn-secondary" @click="$emit('cancel')">Cancel</button>
        <button
          class="btn btn-primary danger"
          :disabled="typed !== confirmWord || busy"
          @click="$emit('confirm')"
        >
          {{ confirmLabel }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.scrim {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgb(0 0 0 / 0.55);
  padding: var(--s-8);
  z-index: 10;
}

.dialog {
  width: min(440px, 100%);
  background: var(--surface-raised);
  border-radius: var(--r-lg);
  box-shadow: var(--e-2);
  padding: var(--s-8);
}

h2 { font-size: var(--t-lg); font-weight: 500; margin-bottom: var(--s-4); }

.body {
  font-size: var(--t-base);
  line-height: 1.55;
  color: var(--text-muted);
  margin-bottom: var(--s-6);
  text-wrap: pretty;
}

.prompt { display: block; font-size: var(--t-sm); color: var(--text-dim); margin-bottom: var(--s-2); }

.row { display: flex; justify-content: flex-end; gap: var(--s-3); margin-top: var(--s-6); }

.danger { border-color: var(--blocked); color: var(--blocked); }
.danger:hover:not(:disabled) { background: var(--blocked-tint); }
</style>
