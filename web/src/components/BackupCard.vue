<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { CircleAlert, CircleCheck, HardDrive, LoaderCircle } from 'lucide-vue-next'
import { api, type BackupEvery, type BackupState } from '@/lib/api'
import { formatDate, formatEpoch } from '@/lib/format'
import { fieldClass, segmentClass } from '@/lib/utils'
import Card from '@/components/ui/Card.vue'
import Button from '@/components/ui/Button.vue'

// Automatic backups into a folder: an external drive, a NAS, or a folder that
// Dropbox, OneDrive or Google Drive syncs. Each run copies only new mail.

const { t } = useI18n()
const state = ref<BackupState | null>(null)
const dir = ref('')
const every = ref<BackupEvery>('off')
const saving = ref(false)
let poll: number | undefined

const EVERY: BackupEvery[] = ['off', 'daily', 'weekly']
const dirty = computed(() => !!state.value && (dir.value.trim() !== state.value.dir || every.value !== state.value.every))
const epochISO = (s?: number) => (s ? new Date(s * 1000).toISOString() : '')

function apply(s: BackupState) {
  state.value = s
  dir.value = s.dir
  every.value = s.every
}

async function refresh(keepEdits = false) {
  try {
    const s = await api.getBackup()
    if (keepEdits) state.value = s
    else apply(s)
    window.clearTimeout(poll)
    if (s.running) poll = window.setTimeout(() => refresh(true), 1500)
  } catch (e) {
    toast.error(t('backup.load_error'), { description: String(e) })
  }
}
onMounted(() => refresh())
onUnmounted(() => window.clearTimeout(poll))

async function save() {
  saving.value = true
  try {
    apply(await api.saveBackup({ dir: dir.value.trim(), every: every.value }))
    toast.success(t('backup.saved'))
  } catch (e) {
    toast.error(t('backup.save_error'), { description: String(e) })
  } finally {
    saving.value = false
  }
}

async function runNow() {
  try {
    if (dirty.value) await save()
    await api.runBackup()
    await refresh(true)
  } catch (e) {
    toast.error(t('backup.run_error'), { description: String(e) })
  }
}
</script>

<template>
  <Card class="p-6 space-y-4" data-testid="backup-card">
    <div class="flex items-start gap-3">
      <HardDrive class="mt-1 h-5 w-5 shrink-0 text-muted-foreground" />
      <div>
        <h3 class="text-lg font-semibold mb-1">{{ t('backup.title') }}</h3>
        <p class="text-sm text-muted-foreground">{{ t('backup.hint') }}</p>
      </div>
    </div>

    <p v-if="state && !state.available" class="text-sm text-muted-foreground">{{ t('backup.temporary') }}</p>

    <template v-else-if="state">
      <label class="block space-y-1">
        <span class="text-sm">{{ t('backup.folder') }}</span>
        <input
          v-model="dir"
          :class="fieldClass"
          :placeholder="state.default_dir"
          spellcheck="false"
          autocomplete="off"
          data-testid="backup-dir"
        />
        <span class="block text-xs text-muted-foreground">{{ t('backup.folder_hint') }}</span>
      </label>

      <div class="flex flex-wrap items-center gap-3">
        <span class="text-sm">{{ t('backup.every') }}</span>
        <div class="inline-flex gap-1 p-1 rounded-lg bg-muted" role="radiogroup" :aria-label="t('backup.every')">
          <button
            v-for="e in EVERY"
            :key="e"
            type="button"
            role="radio"
            :aria-checked="every === e"
            :class="segmentClass(every === e)"
            :data-testid="`backup-every-${e}`"
            @click="every = e"
          >
            {{ t(`backup.every_${e}`) }}
          </button>
        </div>
        <div class="ml-auto flex gap-2">
          <Button variant="outline" :disabled="saving || state.running || !(dir.trim() || state.dir)" data-testid="backup-run" @click="runNow">
            {{ t('backup.run_now') }}
          </Button>
          <Button :disabled="saving || !dirty" data-testid="backup-save" @click="save">{{ t('backup.save') }}</Button>
        </div>
      </div>

      <div class="rounded-lg border border-hairline bg-muted/40 p-3 text-sm" data-testid="backup-status" aria-live="polite">
        <p v-if="state.running" class="flex items-center gap-2">
          <LoaderCircle class="h-4 w-4 animate-spin" />{{ t('backup.running') }}
        </p>
        <template v-else-if="state.last.at">
          <p v-if="state.last.ok" class="flex items-center gap-2">
            <CircleCheck class="h-4 w-4 shrink-0 text-green-600" />
            <span :title="formatEpoch(state.last.at)">
              {{ t('backup.last_ok', { when: formatDate(epochISO(state.last.at)), added: state.last.added, total: state.last.total }, state.last.added) }}
            </span>
          </p>
          <template v-else>
            <p class="flex items-center gap-2 text-destructive">
              <CircleAlert class="h-4 w-4 shrink-0" />
              <span>{{ t('backup.last_failed', { when: formatDate(epochISO(state.last.at)) }) }}</span>
            </p>
            <p class="mt-1 break-words font-mono text-xs text-muted-foreground">{{ state.last.error }}</p>
            <p class="mt-1 text-xs text-muted-foreground">
              {{ state.last.last_ok ? t('backup.last_good', { when: formatDate(epochISO(state.last.last_ok)) }) : t('backup.never_ok') }}
            </p>
          </template>
          <p v-if="state.last.failed" class="mt-1 text-xs text-muted-foreground">{{ t('backup.some_failed', { count: state.last.failed }) }}</p>
        </template>
        <p v-else class="text-muted-foreground">{{ t('backup.never') }}</p>
        <p v-if="state.every !== 'off' && state.dir && !state.running" class="mt-1 text-xs text-muted-foreground">
          {{ t(`backup.schedule_${state.every}`) }}
        </p>
      </div>
    </template>
  </Card>
</template>
