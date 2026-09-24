<script setup>
import TaskDetail from '../components/TaskDetail.vue'

// The task at its own URL. This is what a link in a commit message points at
// and what an agent is told to read, so it stays a real page rather than
// becoming a state of the board -- the drawer on the board is a second way in,
// not a replacement for this one.
defineProps({ taskRef: String })
</script>

<template>
  <TaskDetail :task-ref="taskRef" />
</template>
