<script setup lang="ts">
import { computed, watchEffect } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Toaster } from 'vue-sonner'
import 'vue-sonner/style.css'
import { APP_NAME } from '@/lib/app'

const { t } = useI18n()
const route = useRoute()

const chromeless = computed(() => route.meta.chromeless === true)

const nav = computed(() => [
  { to: '/', label: t('nav.library') },
  { to: '/import', label: t('nav.import') },
  { to: '/settings', label: t('nav.settings') },
])

const pageTitle = computed(() => {
  const key = route.meta.titleKey as string | undefined
  return key ? t(key) : ''
})

watchEffect(() => {
  // Chrome-less detail windows set their own title (the email subject)
  // from inside EmailDetail; skip the app-level title there.
  if (chromeless.value) return
  document.title = pageTitle.value
    ? `${APP_NAME} | ${pageTitle.value}`
    : APP_NAME
})
</script>

<template>
  <div v-if="chromeless" class="min-h-screen px-6 py-6">
    <RouterView />
  </div>

  <div v-else class="min-h-screen flex flex-col">
    <header class="bg-black text-white">
      <div class="max-w-[1800px] mx-auto px-6 h-11 flex items-center gap-6 text-xs">
        <RouterLink to="/" class="flex items-center gap-2 font-semibold tracking-tight text-white/90 hover:text-white">
          <img src="/icon-64.png" srcset="/favicon.png 1x, /icon-64.png 2x" alt="" class="h-6 w-6 rounded-sm" />
          <span>{{ APP_NAME }}</span>
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
  </div>

  <Toaster position="bottom-right" rich-colors close-button />
</template>
