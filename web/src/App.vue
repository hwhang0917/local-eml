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
    <header class="border-b bg-card">
      <div class="max-w-[1800px] mx-auto px-4 h-14 flex items-center gap-6">
        <RouterLink to="/" class="font-semibold tracking-tight">local-eml</RouterLink>
        <nav class="flex items-center gap-1 text-sm">
          <RouterLink
            v-for="r in nav"
            :key="r.to"
            :to="r.to"
            class="px-3 py-1.5 rounded-md hover:bg-accent hover:text-accent-foreground"
            active-class="bg-accent text-accent-foreground"
          >
            {{ r.label }}
          </RouterLink>
        </nav>
        <select
          :value="locale"
          @change="changeLocale"
          class="ml-auto h-8 rounded-md border bg-background px-2 text-sm cursor-pointer"
          :title="t('nav.settings')"
        >
          <option value="en">English</option>
          <option value="ko">한국어</option>
        </select>
      </div>
    </header>
    <main class="flex-1 max-w-[1800px] mx-auto w-full px-4 py-6">
      <RouterView />
    </main>
  </div>
</template>
