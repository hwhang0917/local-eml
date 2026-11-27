<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import {
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogOverlay,
  AlertDialogPortal,
  AlertDialogRoot,
  AlertDialogTitle,
} from 'reka-ui'
import Button from '@/components/ui/Button.vue'
import { fieldClass } from '@/lib/utils'

const props = defineProps<{
  title: string
  description?: string
  label: string
  confirmLabel: string
  cancelLabel: string
  /** Prefilled (and selected) each time the dialog opens. */
  initialValue?: string
  placeholder?: string
  /** Keeps the dialog open with its actions disabled while the caller works. */
  busy?: boolean
}>()
const open = defineModel<boolean>('open', { required: true })
// The dialog does not close itself on submit: the caller closes it once the
// work succeeds, so a failed save leaves the typed value in place to retry.
const emit = defineEmits<{ submit: [value: string] }>()

const value = ref('')
const input = ref<HTMLInputElement | null>(null)
const canSubmit = computed(() => value.value.trim().length > 0 && !props.busy)

watch(open, async (o) => {
  if (!o) return
  value.value = props.initialValue ?? ''
  await nextTick()
  input.value?.focus()
  input.value?.select()
})

// reka's AlertDialog always moves focus to its Cancel button a tick after it
// opens, which beat the open watch below and left the name field unfocused.
// Taking focus back after that tick puts the caret where the person types.
function focusInput(e: Event) {
  e.preventDefault()
  setTimeout(() => {
    input.value?.focus()
    input.value?.select()
  }, 0)
}

function submit() {
  if (canSubmit.value) emit('submit', value.value.trim())
}
</script>

<!--
  Modal text prompt replacing window.prompt. Same shell as ConfirmDialog:
  focus is trapped, Escape cancels, the backdrop does not dismiss. Enter
  submits through the form.
-->
<template>
  <AlertDialogRoot v-model:open="open">
    <AlertDialogPortal>
      <AlertDialogOverlay
        class="fixed inset-0 z-50 bg-black/40 backdrop-blur-[2px]
          data-[state=open]:animate-in data-[state=open]:fade-in-0
          data-[state=closed]:animate-out data-[state=closed]:fade-out-0"
      />
      <AlertDialogContent
        class="fixed left-1/2 top-1/2 z-50 w-[min(28rem,calc(100vw-2rem))] -translate-x-1/2 -translate-y-1/2
          rounded-xl border border-hairline bg-background p-6 shadow-xl outline-none
          data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95
          data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95"
        @open-auto-focus="focusInput"
      >
        <form @submit.prevent="submit">
          <AlertDialogTitle class="text-base font-semibold">{{ title }}</AlertDialogTitle>
          <AlertDialogDescription v-if="description" class="mt-2 text-sm text-muted-foreground">
            {{ description }}
          </AlertDialogDescription>
          <label class="mt-4 block space-y-1">
            <span class="text-sm">{{ label }}</span>
            <input
              ref="input"
              v-model="value"
              :class="fieldClass"
              :placeholder="placeholder"
              :disabled="busy"
              autocomplete="off"
              data-testid="prompt-dialog-input"
            />
          </label>
          <div class="mt-5 flex justify-end gap-2">
            <AlertDialogCancel as-child>
              <Button type="button" variant="outline" size="sm" :disabled="busy">{{ cancelLabel }}</Button>
            </AlertDialogCancel>
            <Button type="submit" size="sm" :disabled="!canSubmit" data-testid="prompt-dialog-confirm">
              {{ confirmLabel }}
            </Button>
          </div>
        </form>
      </AlertDialogContent>
    </AlertDialogPortal>
  </AlertDialogRoot>
</template>
