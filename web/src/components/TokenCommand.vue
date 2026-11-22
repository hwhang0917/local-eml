<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useClipboard } from '@vueuse/core'
import { Check, Copy } from 'lucide-vue-next'
import { toast } from 'vue-sonner'

// Sign-in help for people who have never opened a terminal: which app to open
// on their OS, how to find it, and a one-click copy of the command. The token
// lives only in a file the server's owner can read, so the page can't fetch
// it; running the command is the only way short of the launcher.

const COMMAND = 'local-eml token'

type OS = 'windows' | 'mac' | 'other'

function detectOS(): OS {
  const nav = navigator as Navigator & { userAgentData?: { platform?: string } }
  const p = (nav.userAgentData?.platform || navigator.platform || navigator.userAgent).toLowerCase()
  if (p.includes('win')) return 'windows'
  if (p.includes('mac')) return 'mac'
  return 'other'
}

const { t } = useI18n()
const os = detectOS()
const app = computed(() => t(`auth.app_${os}`))
const howTo = computed(() => t(`auth.open_${os}`))

// legacy: falls back to execCommand when the Clipboard API is unavailable.
const { copy, copied } = useClipboard({ legacy: true, copiedDuring: 2000 })

async function copyCommand() {
  try {
    await copy(COMMAND)
  } catch {
    toast.error(t('auth.copy_failed'))
  }
}
</script>

<template>
  <div class="w-full max-w-md rounded-sm border border-hairline p-4 text-left text-sm break-keep">
    <p class="font-medium mb-3">{{ t('auth.help_title') }}</p>
    <ol class="list-decimal pl-5 space-y-3 text-muted-foreground">
      <li>
        {{ t('auth.step_open', { app }) }}
        <span v-if="howTo" class="block text-xs mt-0.5">{{ howTo }}</span>
      </li>
      <li>
        {{ t('auth.step_run') }}
        <div class="mt-2 flex items-center gap-2 rounded-sm bg-muted px-3 py-2">
          <code class="flex-1 font-mono text-foreground select-all">{{ COMMAND }}</code>
          <button
            type="button"
            class="inline-flex items-center gap-1.5 h-7 px-2.5 rounded-sm border border-hairline bg-background
              text-xs text-foreground hover:bg-accent focus-visible:outline-none focus-visible:ring-2
              focus-visible:ring-ring"
            :aria-label="t('auth.copy')"
            @click="copyCommand"
          >
            <Check v-if="copied" class="h-3.5 w-3.5" aria-hidden="true" />
            <Copy v-else class="h-3.5 w-3.5" aria-hidden="true" />
            <span aria-live="polite">{{ copied ? t('auth.copied') : t('auth.copy') }}</span>
          </button>
        </div>
      </li>
      <li>{{ t('auth.step_paste') }}</li>
    </ol>
  </div>
</template>
