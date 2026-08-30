<script setup>
import StatusCheck from './icons/StatusCheck.vue'

// Shape carries the position on the pipeline; luminance carries the demand.
// Circle = on the pipeline. Check = left it. Diamond = fell off the side.
// Only review and blocked are allowed to be bright.
defineProps({
  status: { type: String, required: true },
  label: { type: Boolean, default: true },
})
</script>

<template>
  <span class="mark" :data-status="status">
    <StatusCheck v-if="status === 'done'" class="glyph-check" />
    <span v-else class="glyph" />
    <span v-if="label" class="label">{{ status }}</span>
  </span>
</template>

<style scoped>
.mark {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  font-size: 12.5px;
  white-space: nowrap;
}

.glyph {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  flex: none;
  transition: background var(--motion), box-shadow var(--motion);
}

.glyph-check { flex: none; }

.label { transition: color var(--motion); }

[data-status='backlog'] .glyph { box-shadow: inset 0 0 0 1.5px #595d6c; }
[data-status='backlog'] .label { color: #9397ab; }

[data-status='queue'] .glyph { box-shadow: inset 0 0 0 1.5px #9397ab; }
[data-status='queue'] .label { color: #cfd3e5; }

[data-status='active'] .glyph { background: #968ae0; }
[data-status='active'] .label { color: #e4e7f5; }

/* The only glow anywhere in the app. A task in review is always waiting on the
   person looking at the screen, so it is the one thing worth spending it on. */
[data-status='review'] .glyph {
  background: var(--accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 22%, transparent);
}
[data-status='review'] .label { color: var(--accent); }

[data-status='done'] { color: #75798c; }
[data-status='done'] .label { color: #75798c; }

/* blocked is not a stage in the pipeline, so it does not get a circle. */
[data-status='blocked'] .glyph {
  width: 8px;
  height: 8px;
  border-radius: 0;
  background: var(--blocked);
  transform: rotate(45deg);
}
[data-status='blocked'] .label { color: var(--blocked); }
</style>
