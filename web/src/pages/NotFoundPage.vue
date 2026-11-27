<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { FileQuestion } from 'lucide-vue-next'
import ErrorState from '@/components/ErrorState.vue'
import { buttonClass } from '@/lib/utils'

// Shown for any address the app has no page for. The path itself is never
// echoed back: it is whatever someone typed or linked, not something to
// render.
const props = defineProps<{ kind?: 'settings' }>()
const { t } = useI18n()

const settings = computed(() => props.kind === 'settings')
</script>

<template>
  <ErrorState
    :icon="FileQuestion"
    :title="settings ? t('not_found.settings_title') : t('not_found.title')"
    :body="settings ? t('not_found.settings_body') : t('not_found.body')"
  >
    <RouterLink v-if="settings" :to="{ name: 'settings-about' }" :class="buttonClass('secondary')">
      {{ t('not_found.to_settings') }}
    </RouterLink>
    <RouterLink to="/" :class="buttonClass()">{{ t('not_found.to_library') }}</RouterLink>
  </ErrorState>
</template>
