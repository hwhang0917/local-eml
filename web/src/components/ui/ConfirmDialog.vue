<script setup lang="ts">
import {
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogOverlay,
  AlertDialogPortal,
  AlertDialogRoot,
  AlertDialogTitle,
} from 'reka-ui'
import Button from '@/components/ui/Button.vue'

defineProps<{
  title: string
  description: string
  confirmLabel: string
  cancelLabel: string
  destructive?: boolean
}>()
const open = defineModel<boolean>('open', { required: true })
const emit = defineEmits<{ confirm: [] }>()
</script>

<!--
  Modal yes/no prompt replacing window.confirm. AlertDialog (not Dialog) so
  focus is trapped, Escape cancels, and clicking the backdrop does not
  dismiss — the actions are the only way out, which is what a confirm is.
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
      >
        <AlertDialogTitle class="text-base font-semibold">{{ title }}</AlertDialogTitle>
        <AlertDialogDescription class="mt-2 text-sm text-muted-foreground">
          {{ description }}
        </AlertDialogDescription>
        <div class="mt-5 flex justify-end gap-2">
          <AlertDialogCancel as-child>
            <Button variant="outline" size="sm">{{ cancelLabel }}</Button>
          </AlertDialogCancel>
          <AlertDialogAction as-child>
            <Button :variant="destructive ? 'destructive' : 'default'" size="sm" @click="emit('confirm')">
              {{ confirmLabel }}
            </Button>
          </AlertDialogAction>
        </div>
      </AlertDialogContent>
    </AlertDialogPortal>
  </AlertDialogRoot>
</template>
