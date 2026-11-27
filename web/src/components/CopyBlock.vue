<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useClipboard } from '@vueuse/core'
import { Check, Copy } from 'lucide-vue-next'
import { toast } from 'vue-sonner'

// A block of text to paste somewhere else (a command, a config file). `shown`
// is what the page displays and `text` what the button copies, so a secret
// can stay masked on screen and still be copied whole.
const props = defineProps<{ text: string; shown?: string; label?: string }>()

const { t } = useI18n()
// legacy: falls back to execCommand when the Clipboard API is unavailable.
const { copy, copied } = useClipboard({ legacy: true, copiedDuring: 2000 })

async function onCopy() {
  try {
    await copy(props.text)
  } catch {
    toast.error(t('auth.copy_failed'))
  }
}
</script>

<template>
  <div class="relative rounded-sm bg-muted">
    <pre
      class="px-3 py-2.5 pr-24 font-mono text-xs leading-relaxed text-foreground whitespace-pre-wrap break-all"
    >{{ shown ?? text }}</pre>
    <button
      type="button"
      class="absolute right-2 top-2 inline-flex items-center gap-1.5 h-7 px-2.5 rounded-sm border border-hairline
        bg-background text-xs text-foreground hover:bg-accent focus-visible:outline-none focus-visible:ring-2
        focus-visible:ring-ring"
      :aria-label="label ? `${t('auth.copy')}: ${label}` : t('auth.copy')"
      @click="onCopy"
    >
      <Check v-if="copied" class="h-3.5 w-3.5" aria-hidden="true" />
      <Copy v-else class="h-3.5 w-3.5" aria-hidden="true" />
      <span aria-live="polite">{{ copied ? t('auth.copied') : t('auth.copy') }}</span>
    </button>
  </div>
</template>
