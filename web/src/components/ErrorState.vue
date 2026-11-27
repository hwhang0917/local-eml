<script setup lang="ts">
import type { Component } from 'vue'

// A centred "nothing here" / "something broke" panel, in the same shape as
// the sign-in and narrow-window notices in App.vue. Actions go in the slot.
defineProps<{
  icon: Component
  title: string
  body?: string
  detail?: string
}>()
</script>

<template>
  <div class="flex flex-col items-center justify-center gap-4 px-6 py-24 text-center" role="alert">
    <component :is="icon" class="h-10 w-10 text-muted-foreground" aria-hidden="true" />
    <h1 class="text-lg font-semibold">{{ title }}</h1>
    <p v-if="body" class="max-w-md text-sm text-muted-foreground text-balance break-keep">{{ body }}</p>
    <p v-if="detail" class="max-w-xl text-xs text-muted-foreground font-mono break-all">{{ detail }}</p>
    <div class="mt-2 flex flex-wrap items-center justify-center gap-2">
      <slot />
    </div>
  </div>
</template>
