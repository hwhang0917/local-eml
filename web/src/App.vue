<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, RouterView } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Toaster } from 'vue-sonner'
import 'vue-sonner/style.css'

const { t } = useI18n()

const nav = computed(() => [
  { to: '/', label: t('nav.library') },
  { to: '/import', label: t('nav.import') },
  { to: '/settings', label: t('nav.settings') },
])
</script>

<template>
  <div class="min-h-screen flex flex-col">
    <header class="bg-black text-white">
      <div class="max-w-[1800px] mx-auto px-6 h-11 flex items-center gap-6 text-xs">
        <RouterLink to="/" class="flex items-center gap-2 font-semibold tracking-tight text-white/90 hover:text-white">
          <img src="/icon-64.png" srcset="/favicon.png 1x, /icon-64.png 2x" alt="" class="h-6 w-6 rounded-sm" />
          <span>local-eml</span>
        </RouterLink>
        <nav class="flex items-center gap-1">
          <RouterLink
            v-for="r in nav"
            :key="r.to"
            :to="r.to"
            class="px-3 py-1 rounded-xs text-white/70 hover:text-white"
            active-class="text-white"
          >
            {{ r.label }}
          </RouterLink>
        </nav>
      </div>
    </header>
    <main class="flex-1 max-w-[1800px] mx-auto w-full px-6 py-10">
      <RouterView />
    </main>
    <Toaster position="bottom-right" rich-colors close-button />
  </div>
</template>
