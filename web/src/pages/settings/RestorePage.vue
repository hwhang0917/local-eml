<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { api } from '@/lib/api'
import type { RestoreSummary } from '@/lib/api'
import Card from '@/components/ui/Card.vue'
import Button from '@/components/ui/Button.vue'

const { t } = useI18n()

const inputClass =
  'w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-primary'

const restoreFile = ref<File | null>(null)
const restoreBusy = ref(false)
const restoreResult = ref<RestoreSummary | null>(null)

function onRestoreFileChange(e: Event) {
  restoreFile.value = (e.target as HTMLInputElement).files?.[0] ?? null
  restoreResult.value = null
}

async function runRestore() {
  const f = restoreFile.value
  if (!f || restoreBusy.value) return
  if (!window.confirm(t('restore.confirm'))) return
  restoreBusy.value = true
  try {
    restoreResult.value = await api.restoreBackup(f)
    toast.success(t('restore.done_title'))
  } catch (e) {
    toast.error(t('restore.error'), { description: String(e) })
  } finally {
    restoreBusy.value = false
  }
}
</script>

<template>
  <Card class="p-6 space-y-4">
    <div>
      <h3 class="text-lg font-semibold mb-1">{{ t('restore.title') }}</h3>
      <p class="text-sm text-muted-foreground">{{ t('restore.hint') }}</p>
    </div>

    <input
      type="file"
      accept=".db,.zip"
      :class="inputClass"
      class="cursor-pointer"
      @change="onRestoreFileChange"
    />

    <p class="text-sm text-muted-foreground">{{ t('restore.password_note') }}</p>

    <div v-if="restoreResult" class="text-sm rounded-md border border-border p-3 space-y-1">
      <p class="font-medium">{{ t('restore.done_title') }}</p>
      <p class="text-muted-foreground">
        {{ t('restore.summary', {
          emails: restoreResult.emails,
          categories: restoreResult.categories,
          settings: restoreResult.settings,
          profiles: restoreResult.imap_profiles + restoreResult.s3_profiles,
        }) }}
      </p>
    </div>

    <div class="flex justify-end">
      <Button :disabled="!restoreFile || restoreBusy" @click="runRestore">
        {{ restoreBusy ? t('restore.busy') : t('restore.start') }}
      </Button>
    </div>
  </Card>
</template>
