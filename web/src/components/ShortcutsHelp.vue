<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { hasModifier, isTypingTarget } from '@/lib/keys'
import Button from '@/components/ui/Button.vue'

const { t } = useI18n()
const dialog = ref<HTMLDialogElement | null>(null)

const rows = computed(() => [
  { keys: 'j / k', label: t('shortcuts.nav') },
  { keys: 'Enter', label: t('shortcuts.open') },
  { keys: 's', label: t('shortcuts.star') },
  { keys: '/', label: t('shortcuts.search') },
  { keys: '← / →', label: t('shortcuts.prev_next') },
  { keys: 'Esc', label: t('shortcuts.escape') },
  { keys: '?', label: t('shortcuts.help') },
])

function onKeydown(e: KeyboardEvent) {
  if (e.key !== '?' || isTypingTarget(e) || hasModifier(e)) return
  e.preventDefault()
  dialog.value?.showModal()
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <dialog
    ref="dialog"
    aria-labelledby="shortcuts-title"
    class="m-auto w-[90vw] max-w-sm rounded-lg border bg-background p-0 text-foreground backdrop:bg-black/50"
  >
    <div class="space-y-3 p-5">
      <h2 id="shortcuts-title" class="font-semibold">{{ t('shortcuts.title') }}</h2>
      <dl class="grid grid-cols-[auto_1fr] items-center gap-x-4 gap-y-2 text-sm">
        <template v-for="r in rows" :key="r.keys">
          <dt>
            <kbd class="rounded-sm border border-hairline bg-accent px-1.5 py-0.5 font-mono text-xs">{{ r.keys }}</kbd>
          </dt>
          <dd class="text-muted-foreground">{{ r.label }}</dd>
        </template>
      </dl>
      <div class="flex justify-end">
        <Button variant="ghost" size="sm" @click="dialog?.close()">
          {{ t('shortcuts.close') }}
        </Button>
      </div>
    </div>
  </dialog>
</template>
