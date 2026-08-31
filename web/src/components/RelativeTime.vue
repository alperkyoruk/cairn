<script setup>
import { computed } from 'vue'
import { formatRelative, formatAbsolute, formatFull } from '../composables/useRelativeTime.js'

const props = defineProps({
  value: String,
  absolute: { type: Boolean, default: false },
  // Renders "4m ago" rather than "4m". Under a minute the convention is the
  // bare word "now", which does not take a suffix -- "now ago" is not English.
  ago: { type: Boolean, default: false },
})

const shown = computed(() => {
  if (props.absolute) return formatAbsolute(props.value)
  const relative = formatRelative(props.value)
  if (!props.ago || relative === 'now') return relative
  return `${relative} ago`
})
</script>

<template>
  <time :datetime="value" :title="formatFull(value)">{{ shown }}</time>
</template>

<style scoped>
time { font-variant-numeric: tabular-nums; }
</style>
