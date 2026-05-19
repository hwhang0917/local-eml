<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, RouterView } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { setLocale, type Locale } from '@/i18n'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select'

const { t, locale } = useI18n()

const nav = computed(() => [
  { to: '/', label: t('nav.library') },
  { to: '/import', label: t('nav.import') },
  { to: '/settings', label: t('nav.settings') },
])

const langs = [
  { value: 'en' as Locale, label: 'English' },
  { value: 'ko' as Locale, label: '한국어' },
]
const currentLangLabel = computed(
  () => langs.find((l) => l.value === locale.value)?.label ?? locale.value,
)
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
        <Select
          :model-value="locale"
          @update:model-value="(v) => v && setLocale(v as Locale)"
          class="ml-auto"
        >
          <SelectTrigger
            class="h-7 border-white/15 bg-white/5 text-white/90 hover:bg-white/10 text-xs"
          >
            <SelectValue>{{ currentLangLabel }}</SelectValue>
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="l in langs" :key="l.value" :value="l.value">
              {{ l.label }}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>
    </header>
    <main class="flex-1 max-w-[1800px] mx-auto w-full px-6 py-10">
      <RouterView />
    </main>
  </div>
</template>
