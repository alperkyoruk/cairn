<script setup>
defineProps({ status: String })
</script>

<template>
  <span class="badge" :data-status="status">{{ status }}</span>
</template>

<style scoped>
/* blocked is deliberately the only filled badge: it is not a stage in the
   pipeline, it is the one that needs someone to do something. */
.badge {
  display: inline-block;
  font-size: var(--text-xs);
  font-family: var(--font-mono);
  padding: 1px var(--space-2);
  border-radius: 3px;
  border: 1px solid currentColor;
  white-space: nowrap;
}
.badge[data-status='backlog'] { color: var(--status-backlog); }
.badge[data-status='queue']   { color: var(--status-queue); }
.badge[data-status='active']  { color: var(--status-active); }
.badge[data-status='review']  { color: var(--status-review); }
.badge[data-status='done']    { color: var(--status-done); }
.badge[data-status='blocked'] {
  color: var(--bg-raised);
  background: var(--status-blocked);
  border-color: var(--status-blocked);
}
</style>
