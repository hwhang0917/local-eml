<script setup lang="ts">
import EmailDetail from '@/components/EmailDetail.vue'

defineProps<{ sha: string }>()
</script>

<template>
  <EmailDetail :sha="sha" standalone />
</template>
