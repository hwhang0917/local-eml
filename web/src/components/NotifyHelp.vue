<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleHelp } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { cn, dialogClass, segmentClass } from '@/lib/utils'
import { useNewMailNotifier } from '@/composables/useNewMailNotifier'
import Button from '@/components/ui/Button.vue'

// "Notifications don't show up" help. The browser part usually works; what
// hides the pop-up is an OS setting (Do Not Disturb, Focus, or banners turned
// off for the browser), so the steps are per OS. Linux desktops vary too much
// to list, and their users tend to know where the switch is.

type OS = 'windows' | 'mac'

function detectOS(): OS {
  const nav = navigator as Navigator & { userAgentData?: { platform?: string } }
  const p = (nav.userAgentData?.platform || navigator.platform || navigator.userAgent).toLowerCase()
  return p.includes('mac') ? 'mac' : 'windows'
}

const { t } = useI18n()
const notifier = useNewMailNotifier()
const dialog = ref<HTMLDialogElement | null>(null)
const os = ref<OS>(detectOS())

const STEP_COUNT = 3
const steps = computed(() =>
  Array.from({ length: STEP_COUNT }, (_, i) => ({
    title: t(`import.notify_faq_${os.value}_${i + 1}_title`),
    body: t(`import.notify_faq_${os.value}_${i + 1}_body`),
  })),
)

function open() {
  dialog.value?.showModal()
}

function sendTest() {
  if (!notifier.sendTest()) toast.warning(t('import.notify_faq_test_off'))
}
</script>

<template>
  <button
    type="button"
    class="inline-flex align-middle ml-1 text-muted-foreground hover:text-foreground cursor-pointer"
    :title="t('import.notify_faq_open')"
    :aria-label="t('import.notify_faq_open')"
    data-testid="notify-help"
    @click.prevent.stop="open"
  >
    <CircleHelp class="h-3.5 w-3.5" />
  </button>
  <!-- The trigger sits inside the notification checkbox's <label>. A click
       anywhere inside a label activates its checkbox (a default action that
       stopPropagation can't block), so the dialog lives under <body>. -->
  <Teleport to="body">
    <dialog
      ref="dialog"
      aria-labelledby="notify-help-title"
      :class="cn(dialogClass, 'max-w-lg')"
    >
      <div class="space-y-4 p-5 text-sm break-keep">
        <div class="space-y-1">
          <h2 id="notify-help-title" class="font-semibold text-base">{{ t('import.notify_faq_title') }}</h2>
          <p class="text-muted-foreground">{{ t('import.notify_faq_intro') }}</p>
        </div>
        <div class="inline-flex gap-1 p-1 rounded-lg bg-muted" role="tablist">
          <button
            v-for="o in (['windows', 'mac'] as const)"
            :key="o"
            type="button"
            role="tab"
            :aria-selected="os === o"
            :class="segmentClass(os === o)"
            @click="os = o"
          >
            {{ t(`import.notify_faq_tab_${o}`) }}
          </button>
        </div>
        <ol class="list-decimal pl-5 space-y-3" role="tabpanel">
          <li v-for="(s, i) in steps" :key="i">
            <p class="font-medium">{{ s.title }}</p>
            <p class="text-muted-foreground mt-0.5">{{ s.body }}</p>
          </li>
        </ol>
        <p class="text-muted-foreground">{{ t('import.notify_faq_tab_note') }}</p>
        <div class="flex items-center justify-between gap-2 pt-1">
          <Button variant="outline" size="sm" data-testid="notify-test" @click="sendTest">
            {{ t('import.notify_faq_test') }}
          </Button>
          <Button variant="ghost" size="sm" @click="dialog?.close()">
            {{ t('shortcuts.close') }}
          </Button>
        </div>
      </div>
    </dialog>
  </Teleport>
</template>
