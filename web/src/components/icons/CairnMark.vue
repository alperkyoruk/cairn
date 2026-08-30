<script setup>
defineProps({ size: { type: Number, default: 15 } })
</script>

<template>
  <svg :width="size" :height="size" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
    <path d="M8 1.4 10.4 4H5.6L8 1.4Z" />
    <path d="M4.6 5.2h6.8l1.5 2.6H3.1l1.5-2.6Z" />
    <path d="M2.2 9h11.6l1.6 3.4H.6L2.2 9Z" />
  </svg>
</template>
