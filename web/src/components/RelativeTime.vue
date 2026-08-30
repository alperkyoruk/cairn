<script setup>
import { computed } from 'vue'
import { formatRelative, formatAbsolute, formatFull } from '../composables/useRelativeTime.js'

const props = defineProps({
  value: String,
  absolute: { type: Boolean, default: false },
})

const shown = computed(() =>
  props.absolute ? formatAbsolute(props.value) : formatRelative(props.value))
</script>

<template>
  <time :datetime="value" :title="formatFull(value)">{{ shown }}</time>
</template>

<style scoped>
time { font-variant-numeric: tabular-nums; }
</style>
