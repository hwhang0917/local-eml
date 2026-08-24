<script setup lang="ts">
import { computed, nextTick, watchEffect } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { CircleHelp } from 'lucide-vue-next'
import { ConfigProvider } from 'reka-ui'
import { Toaster } from 'vue-sonner'
import 'vue-sonner/style.css'
import { APP_NAME } from '@/lib/app'
import { useHealth } from '@/composables/useHealth'
import { useTour } from '@/composables/useTour'
import ShortcutsHelp from '@/components/ShortcutsHelp.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const { online, checking, retry } = useHealth()
const tour = useTour()

// Two of the steps point at library controls, so the tour has to run from the
// library route or those steps would highlight nothing.
async function replayTour() {
  if (route.name !== 'library') {
    await router.push('/')
    await nextTick()
  }
  tour.start()
}

const nav = computed(() => [
  { to: '/', label: t('nav.library'), tour: 'library' },
  { to: '/import', label: t('nav.import'), tour: 'import' },
  { to: '/export', label: t('nav.export'), tour: 'export' },
  { to: '/stats', label: t('nav.stats'), tour: 'stats' },
  { to: '/calendar', label: t('nav.calendar'), tour: 'calendar' },
  { to: '/settings', label: t('nav.settings'), tour: 'settings' },
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
  <!-- scroll-body off: reka-ui pads the body to replace the vanishing scrollbar
       when a dropdown locks scroll, but scrollbar-gutter already reserves that
       space — the extra padding was itself the layout shift. -->
  <ConfigProvider :scroll-body="false">
  <div class="min-h-screen flex flex-col">
    <div class="sticky top-0 z-50 shadow-sm">
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
              :data-tour="r.tour"
              class="px-3 py-1 rounded-xs text-white/70 hover:text-white"
              active-class="text-white"
            >
              {{ r.label }}
            </RouterLink>
          </nav>
          <button
            type="button"
            data-tour="help"
            :title="t('tour.replay')"
            :aria-label="t('tour.replay')"
            class="ml-auto inline-flex items-center justify-center h-7 w-7 rounded-sm text-white/70
              hover:text-white hover:bg-white/10 focus-visible:outline-none focus-visible:ring-2
              focus-visible:ring-white/60"
            @click="replayTour"
          >
            <CircleHelp class="h-4 w-4" />
          </button>
        </div>
      </header>
    </div>
    <main class="flex-1 max-w-[1800px] mx-auto w-full px-6 py-10">
      <RouterView />
    </main>
  </div>

  <ShortcutsHelp />
  <Toaster position="bottom-right" rich-colors close-button />
  </ConfigProvider>
</template>
