<script setup lang="ts">
import { computed, watchEffect } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Toaster } from 'vue-sonner'
import 'vue-sonner/style.css'
import { APP_NAME } from '@/lib/app'
import { useHealth } from '@/composables/useHealth'

const { t } = useI18n()
const route = useRoute()
const { online, checking, retry } = useHealth()

const nav = computed(() => [
  { to: '/', label: t('nav.library') },
  { to: '/import', label: t('nav.import') },
  { to: '/export', label: t('nav.export') },
  { to: '/settings', label: t('nav.settings') },
])

const pageTitle = computed(() => {
  const key = route.meta.titleKey as string | undefined
  return key ? t(key) : ''
})

watchEffect(() => {
  // EmailDetail owns the title on the viewer route (it writes the subject).
  if (route.name === 'viewer') return
  document.title = pageTitle.value ? `${APP_NAME} | ${pageTitle.value}` : APP_NAME
})
</script>

<template>
  <div class="min-h-screen flex flex-col">
    <div
      v-if="!online"
      class="bg-destructive text-destructive-foreground px-6 py-2 text-sm flex items-center gap-3"
      role="alert"
    >
      <span class="inline-block h-2 w-2 rounded-full bg-current animate-pulse"></span>
      <span class="flex-1">{{ t('app.server_offline') }}</span>
      <button
        type="button"
        :disabled="checking"
        class="px-3 py-1 rounded-sm border border-current/40 hover:bg-current/10 disabled:opacity-50"
        @click="retry"
      >
        {{ checking ? t('app.server_checking') : t('app.server_retry') }}
      </button>
    </div>
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
