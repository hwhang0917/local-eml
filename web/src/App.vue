<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, RouterView } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { setLocale, type Locale } from '@/i18n'

const { t, locale } = useI18n()

const nav = computed(() => [
  { to: '/', label: t('nav.library') },
  { to: '/import', label: t('nav.import') },
  { to: '/settings', label: t('nav.settings') },
])

function changeLocale(e: Event) {
  setLocale((e.target as HTMLSelectElement).value as Locale)
}
</script>

<template>
  <div class="min-h-screen flex flex-col">
    <header class="bg-black text-white">
      <div class="max-w-[1800px] mx-auto px-6 h-11 flex items-center gap-6 text-xs">
        <RouterLink to="/" class="font-semibold tracking-tight text-white/90 hover:text-white">
          local-eml
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
        <select
          :value="locale"
          @change="changeLocale"
          class="ml-auto bg-transparent text-white/80 hover:text-white text-xs border-none focus:outline-none cursor-pointer"
          :title="t('nav.settings')"
        >
          <option class="text-foreground" value="en">English</option>
          <option class="text-foreground" value="ko">한국어</option>
        </select>
      </div>
    </header>
    <main class="flex-1 max-w-[1800px] mx-auto w-full px-6 py-10">
      <RouterView />
    </main>
  </div>
</template>
