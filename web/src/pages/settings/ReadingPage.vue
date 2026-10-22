<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { api, type ReadTrackingExisting } from '@/lib/api'
import { dialogClass } from '@/lib/utils'
import { useReadTracking } from '@/composables/useReadTracking'
import Card from '@/components/ui/Card.vue'
import Button from '@/components/ui/Button.vue'
import ConfirmDialog from '@/components/ui/ConfirmDialog.vue'

const { t } = useI18n()
const { tracking, refresh } = useReadTracking()
const busy = ref(false)
const existingDialog = ref<HTMLDialogElement | null>(null)
const markAllOpen = ref(false)

// The checkbox only reflects the saved state; it changes after the server
// agrees. The first time it is switched on, existing mail needs a starting
// state, and that question is asked once.
function onToggle() {
  if (!tracking.value || busy.value) return
  if (tracking.value.enabled) {
    void apply(false)
  } else if (tracking.value.configured) {
    void apply(true)
  } else {
    existingDialog.value?.showModal()
  }
}

async function apply(enabled: boolean, existing?: ReadTrackingExisting) {
  busy.value = true
  try {
    await api.setReadTracking(enabled, existing)
    await refresh()
    toast.success(t(enabled ? 'reading.on' : 'reading.off'))
    existingDialog.value?.close()
  } catch (e) {
    toast.error(t('reading.error'), { description: String(e) })
  } finally {
    busy.value = false
  }
}

// A bulk archive or resync lands unread; this is the way to catch up without
// opening every message. Same call as the first-time choice, tracking stays on.
async function markAllRead() {
  busy.value = true
  try {
    await api.setReadTracking(true, 'read')
    toast.success(t('reading.mark_all_done'))
  } catch (e) {
    toast.error(t('reading.error'), { description: String(e) })
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="space-y-6">
    <Card class="p-6 space-y-4">
      <h2 class="text-lg font-semibold">{{ t('reading.title') }}</h2>
      <p class="text-sm text-muted-foreground">{{ t('reading.help') }}</p>

      <label class="flex items-start gap-2 text-sm">
        <input
          type="checkbox"
          class="mt-1"
          :checked="tracking?.enabled ?? false"
          :disabled="!tracking || busy"
          @click.prevent="onToggle"
        />
        <span>
          {{ t('reading.toggle') }}
          <span class="block text-xs text-muted-foreground">{{ t('reading.toggle_help') }}</span>
        </span>
      </label>

      <div v-if="tracking?.enabled" class="flex items-center gap-3 border-t pt-4">
        <Button variant="outline" size="sm" :disabled="busy" @click="markAllOpen = true">
          {{ t('reading.mark_all') }}
        </Button>
        <span class="text-xs text-muted-foreground">{{ t('reading.mark_all_help') }}</span>
      </div>
    </Card>

    <ConfirmDialog
      v-model:open="markAllOpen"
      :title="t('reading.mark_all')"
      :description="t('reading.mark_all_confirm')"
      :confirm-label="t('reading.mark_all')"
      :cancel-label="t('reading.cancel')"
      @confirm="markAllRead"
    />

    <dialog ref="existingDialog" aria-labelledby="existing-dialog-title" :class="dialogClass">
      <div class="space-y-3 p-5">
        <h2 id="existing-dialog-title" class="font-semibold">{{ t('reading.existing_title') }}</h2>
        <p class="text-sm text-muted-foreground">{{ t('reading.existing_body') }}</p>
        <div class="flex justify-end gap-2">
          <Button variant="ghost" size="sm" :disabled="busy" @click="existingDialog?.close()">
            {{ t('reading.cancel') }}
          </Button>
          <Button variant="outline" size="sm" :disabled="busy" @click="apply(true, 'unread')">
            {{ t('reading.existing_unread') }}
          </Button>
          <Button size="sm" :disabled="busy" @click="apply(true, 'read')">
            {{ t('reading.existing_read') }}
          </Button>
        </div>
      </div>
    </dialog>
  </div>
</template>
