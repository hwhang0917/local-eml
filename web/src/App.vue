<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'
</script>

<template>
  <div class="min-h-screen flex flex-col">
    <header class="border-b bg-card">
      <div class="max-w-[1800px] mx-auto px-4 h-14 flex items-center gap-6">
        <RouterLink to="/" class="font-semibold tracking-tight">local-eml</RouterLink>
        <nav class="flex items-center gap-1 text-sm">
          <RouterLink
            v-for="r in [
              { to: '/', label: 'Library' },
              { to: '/import', label: 'Import' },
              { to: '/settings', label: 'Settings' },
            ]"
            :key="r.to"
            :to="r.to"
            class="px-3 py-1.5 rounded-md hover:bg-accent hover:text-accent-foreground"
            active-class="bg-accent text-accent-foreground"
          >
            {{ r.label }}
          </RouterLink>
        </nav>
      </div>
    </header>
    <main class="flex-1 max-w-[1800px] mx-auto w-full px-4 py-6">
      <RouterView />
    </main>
  </div>
</template>
