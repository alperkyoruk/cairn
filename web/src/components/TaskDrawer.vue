<script setup>
import { onBeforeUnmount, onMounted } from 'vue'
import TaskDetail from './TaskDetail.vue'

const props = defineProps({ taskRef: { type: String, required: true } })
const emit = defineEmits(['close', 'moved', 'deleted'])

// Escape closes it, which is the gesture people try without being told. It is
// captured at the window rather than on the panel so it works wherever the
// focus happens to be -- including inside one of the transition forms, where
// the alternative is hunting for the close button.
function onKey(event) {
  if (event.key === 'Escape') emit('close')
}
onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <!-- The scrim dims the board without hiding it. Losing your place is the
       thing this exists to prevent, so the column you came from stays where
       it was and stays readable. -->
  <div class="scrim" @click="emit('close')" />

  <aside class="drawer" role="dialog" aria-modal="true" :aria-label="`Task ${props.taskRef}`">
    <header class="head">
      <span class="ref mono">{{ props.taskRef }}</span>
      <RouterLink :to="`/t/${props.taskRef}`" class="full" title="Open as a full page">
        open as page
      </RouterLink>
      <button class="close" aria-label="Close" @click="emit('close')">×</button>
    </header>

    <div class="body">
      <TaskDetail
        :key="props.taskRef"
        :task-ref="props.taskRef"
        embedded
        @moved="emit('moved', $event)"
        @deleted="emit('deleted')"
      />
    </div>
  </aside>
</template>

<style scoped>
.scrim {
  position: fixed;
  inset: 0;
  background: rgb(0 0 0 / 0.45);
  z-index: 10;
}

.drawer {
  position: fixed;
  inset: 0 0 0 auto;
  width: min(620px, 100vw);
  display: flex;
  flex-direction: column;
  background: var(--bg);
  border-left: 1px solid var(--rule-strong);
  box-shadow: -12px 0 32px rgb(0 0 0 / 0.4);
  z-index: 11;
}

.head {
  display: flex;
  align-items: center;
  gap: var(--s-3);
  padding: var(--s-3) var(--s-4);
  border-bottom: 1px solid var(--rule);
}
.ref { font-size: var(--t-sm); color: var(--text-dim); margin-right: auto; }
.full { font-size: var(--t-sm); color: var(--text-dim); }
.full:hover { color: var(--accent); }

.close {
  font: inherit;
  font-size: 20px;
  line-height: 1;
  color: var(--text-dim);
  background: none;
  border: 0;
  padding: 0 var(--s-2);
  cursor: pointer;
  transition: color var(--motion);
}
.close:hover { color: var(--text); }

/* The drawer scrolls, not the board behind it. */
.body { flex: 1; min-height: 0; overflow-y: auto; }

@media (max-width: 640px) {
  .drawer { width: 100vw; border-left: 0; }
}
</style>
