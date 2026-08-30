<script setup>
import AgentChip from './icons/AgentChip.vue'

// Agents get a glyph, the human gets nothing. The distinction is informational,
// not hierarchical: agents are first-class users here, so the mark is quiet --
// no colour difference, no badge, no capitalisation change.
defineProps({
  name: String,
  isAgent: { type: Boolean, default: false },
})
</script>

<template>
  <span v-if="!name" class="faint">—</span>
  <span v-else class="actor">
    <AgentChip v-if="isAgent" />
    <span>{{ name }}</span>
  </span>
</template>

<style scoped>
.actor { display: inline-flex; align-items: center; gap: 5px; }
</style>
