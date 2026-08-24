<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { cn } from '@/lib/utils'
import { useImports } from '@/composables/useImports'
import { api } from '@/lib/api'
import type { RestoreSummary, S3ImportConfig, S3Profile } from '@/lib/api'
import Card from '@/components/ui/Card.vue'
import Button from '@/components/ui/Button.vue'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select'

const { t } = useI18n()
const { runs, exportActive, startS3Export, cancelRun } = useImports()

const NEW_PROFILE = '__new__'

type Mode = 'zip' | 's3' | 'restore'
const mode = ref<Mode>('zip')

function modeBtnClass(m: Mode) {
  return cn(
    'px-4 py-1.5 text-sm rounded-md cursor-pointer transition-colors',
    mode.value === m
      ? 'bg-primary text-primary-foreground'
      : 'text-muted-foreground hover:text-foreground',
  )
}

const inputClass =
  'w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-primary'

// --- ZIP ---
function downloadZip() {
  window.location.assign(api.exportZipURL())
}

// --- Restore ---
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
  if (!window.confirm(t('export.restore_confirm'))) return
  restoreBusy.value = true
  try {
    restoreResult.value = await api.restoreBackup(f)
    toast.success(t('export.restore_done_title'))
  } catch (e) {
    toast.error(t('export.restore_error'), { description: String(e) })
  } finally {
    restoreBusy.value = false
  }
}

// --- S3 ---
const s3 = ref<S3ImportConfig>({
  accessKeyId: '',
  secretAccessKey: '',
  sessionToken: '',
  region: '',
  bucket: '',
  prefix: '',
})
const s3Confirming = ref(false)
const s3Valid = computed(() => s3.value.bucket.trim().length > 0)
const s3CredsLabel = computed(() =>
  s3.value.accessKeyId?.trim() ? t('import.s3_creds_form') : t('import.s3_creds_system'),
)

function reviewS3() {
  if (s3Valid.value) s3Confirming.value = true
}
function cancelS3() {
  s3Confirming.value = false
}
async function confirmS3() {
  const cfg: S3ImportConfig = {
    bucket: s3.value.bucket.trim(),
    prefix: s3.value.prefix?.trim() || undefined,
    region: s3.value.region?.trim() || undefined,
    accessKeyId: s3.value.accessKeyId?.trim() || undefined,
    secretAccessKey: s3.value.secretAccessKey || undefined,
    sessionToken: s3.value.sessionToken || undefined,
  }
  s3Confirming.value = false
  await startS3Export(cfg)
}

// --- S3 profiles (reused for export) ---
const s3Profiles = ref<S3Profile[]>([])
const selectedS3ProfileId = ref<number | null>(null)

const activeS3Profile = computed(() =>
  s3Profiles.value.find((p) => p.id === selectedS3ProfileId.value) ?? null,
)
const canSaveS3Profile = computed(() => s3.value.bucket.trim().length > 0)

async function loadS3Profiles() {
  try {
    s3Profiles.value = await api.listS3Profiles()
  } catch (e) {
    toast.error(t('import.s3_profile_load_error'), { description: String(e) })
  }
}

function applyS3Profile(p: S3Profile | null) {
  if (!p) {
    s3.value = {
      accessKeyId: '', secretAccessKey: '', sessionToken: '',
      region: '', bucket: '', prefix: '',
    }
    return
  }
  s3.value = {
    accessKeyId: p.access_key_id ?? '',
    secretAccessKey: '',
    sessionToken: '',
    region: p.region ?? '',
    bucket: p.bucket,
    prefix: p.prefix ?? '',
  }
}

function onS3ProfileChange(value: string) {
  if (value === NEW_PROFILE) {
    selectedS3ProfileId.value = null
    applyS3Profile(null)
    return
  }
  const id = Number(value)
  selectedS3ProfileId.value = id
  applyS3Profile(s3Profiles.value.find((p) => p.id === id) ?? null)
}

const s3ProfileSelectValue = computed(() =>
  selectedS3ProfileId.value == null ? NEW_PROFILE : String(selectedS3ProfileId.value),
)
const s3ProfileSelectLabel = computed(() =>
  activeS3Profile.value?.name ?? t('import.s3_profile_new'),
)

async function saveS3Profile() {
  const existing = activeS3Profile.value
  let name = existing?.name ?? ''
  if (!existing) {
    const entered = window.prompt(t('import.s3_profile_save_prompt'), '')
    if (entered == null) return
    name = entered.trim()
    if (!name) return
  }
  try {
    const saved = await api.saveS3Profile({
      name,
      bucket: s3.value.bucket.trim(),
      prefix: s3.value.prefix?.trim() || undefined,
      region: s3.value.region?.trim() || undefined,
      access_key_id: s3.value.accessKeyId?.trim() || undefined,
    })
    await loadS3Profiles()
    selectedS3ProfileId.value = saved.id
  } catch (e) {
    toast.error(t('import.s3_profile_save_error'), { description: String(e) })
  }
}

async function deleteS3Profile() {
  const p = activeS3Profile.value
  if (!p) return
  if (!window.confirm(t('import.s3_profile_delete_confirm', { name: p.name }))) return
  try {
    await api.deleteS3Profile(p.id)
    selectedS3ProfileId.value = null
    applyS3Profile(null)
    await loadS3Profiles()
  } catch (e) {
    toast.error(t('import.s3_profile_delete_error'), { description: String(e) })
  }
}

onMounted(loadS3Profiles)

const exportRuns = computed(() => runs.value.filter((r) => r.kind.endsWith('-export')))
</script>

<template>
  <div class="space-y-6">
    <div class="inline-flex gap-1 p-1 rounded-lg bg-muted">
      <button :class="modeBtnClass('zip')" :aria-pressed="mode === 'zip'" @click="mode = 'zip'">
        {{ t('export.mode_zip') }}
      </button>
      <button :class="modeBtnClass('s3')" :aria-pressed="mode === 's3'" @click="mode = 's3'">
        {{ t('export.mode_s3') }}
      </button>
      <button :class="modeBtnClass('restore')" :aria-pressed="mode === 'restore'" @click="mode = 'restore'">
        {{ t('export.mode_restore') }}
      </button>
    </div>

    <!-- ZIP -->
    <template v-if="mode === 'zip'">
      <Card class="p-6 space-y-4">
        <div>
          <h3 class="text-lg font-semibold mb-1">{{ t('export.zip_title') }}</h3>
          <p class="text-sm text-muted-foreground">{{ t('export.zip_hint') }}</p>
        </div>
        <div class="flex justify-end">
          <Button @click="downloadZip">{{ t('export.zip_download') }}</Button>
        </div>
      </Card>
    </template>

    <!-- Restore -->
    <template v-else-if="mode === 'restore'">
      <Card class="p-6 space-y-4">
        <div>
          <h3 class="text-lg font-semibold mb-1">{{ t('export.restore_title') }}</h3>
          <p class="text-sm text-muted-foreground">{{ t('export.restore_hint') }}</p>
        </div>

        <input
          type="file"
          accept=".db,.zip"
          :class="inputClass"
          class="cursor-pointer"
          @change="onRestoreFileChange"
        />

        <p class="text-sm text-muted-foreground">{{ t('export.restore_password_note') }}</p>

        <div v-if="restoreResult" class="text-sm rounded-md border border-border p-3 space-y-1">
          <p class="font-medium">{{ t('export.restore_done_title') }}</p>
          <p class="text-muted-foreground">
            {{ t('export.restore_summary', {
              emails: restoreResult.emails,
              categories: restoreResult.categories,
              settings: restoreResult.settings,
              profiles: restoreResult.imap_profiles + restoreResult.s3_profiles,
            }) }}
          </p>
        </div>

        <div class="flex justify-end">
          <Button :disabled="!restoreFile || restoreBusy" @click="runRestore">
            {{ restoreBusy ? t('export.restore_busy') : t('export.restore_start') }}
          </Button>
        </div>
      </Card>
    </template>

    <!-- S3 -->
    <template v-else>
      <Card v-if="!s3Confirming" class="p-6 space-y-4">
        <div>
          <h3 class="text-lg font-semibold mb-1">{{ t('export.s3_title') }}</h3>
          <p class="text-sm text-muted-foreground">{{ t('export.s3_hint') }}</p>
        </div>

        <div class="flex items-end gap-2">
          <div class="space-y-1 flex-1 max-w-xs">
            <span class="text-sm">{{ t('import.s3_profile') }}</span>
            <Select
              :model-value="s3ProfileSelectValue"
              @update:model-value="(v) => v && onS3ProfileChange(String(v))"
            >
              <SelectTrigger class="w-full">
                <SelectValue>{{ s3ProfileSelectLabel }}</SelectValue>
              </SelectTrigger>
              <SelectContent>
                <SelectItem :value="NEW_PROFILE">{{ t('import.s3_profile_new') }}</SelectItem>
                <SelectItem v-for="p in s3Profiles" :key="p.id" :value="String(p.id)">
                  {{ p.name }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <Button variant="outline" :disabled="!canSaveS3Profile" @click="saveS3Profile">
            {{ t('import.s3_profile_save') }}
          </Button>
          <Button variant="outline" :disabled="!activeS3Profile" @click="deleteS3Profile">
            {{ t('import.s3_profile_delete') }}
          </Button>
        </div>

        <p class="text-sm text-muted-foreground">{{ t('import.s3_creds_optional') }}</p>
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.s3_access_key') }}</span>
            <input v-model="s3.accessKeyId" :class="inputClass" autocomplete="off" />
          </label>
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.s3_secret_key') }}</span>
            <input v-model="s3.secretAccessKey" type="password" :class="inputClass" autocomplete="off" />
          </label>
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.s3_session_token') }}</span>
            <input v-model="s3.sessionToken" type="password" :class="inputClass" autocomplete="off" />
          </label>
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.s3_region') }}</span>
            <input v-model="s3.region" :class="inputClass" placeholder="us-east-1" autocomplete="off" />
          </label>
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.s3_bucket') }} *</span>
            <input v-model="s3.bucket" :class="inputClass" autocomplete="off" />
          </label>
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.s3_prefix') }}</span>
            <input v-model="s3.prefix" :class="inputClass" placeholder="mail/2026/" autocomplete="off" />
          </label>
        </div>

        <div class="flex justify-end">
          <Button :disabled="!s3Valid" @click="reviewS3">{{ t('import.review') }}</Button>
        </div>
      </Card>

      <Card v-else class="p-6">
        <div class="flex items-start justify-between gap-4 mb-4">
          <div>
            <h3 class="text-lg font-semibold mb-1">{{ t('export.confirm_title') }}</h3>
            <p class="text-sm text-muted-foreground">{{ t('export.s3_confirm_hint') }}</p>
          </div>
          <div class="flex gap-2 shrink-0">
            <Button variant="outline" @click="cancelS3">{{ t('import.cancel') }}</Button>
            <Button :disabled="exportActive" @click="confirmS3">{{ exportActive ? t('export.busy') : t('export.start') }}</Button>
          </div>
        </div>
        <dl class="text-sm space-y-2">
          <div class="flex gap-3">
            <dt class="w-28 text-muted-foreground shrink-0">{{ t('import.s3_bucket') }}</dt>
            <dd class="font-mono">{{ s3.bucket }}</dd>
          </div>
          <div class="flex gap-3">
            <dt class="w-28 text-muted-foreground shrink-0">{{ t('import.s3_prefix') }}</dt>
            <dd class="font-mono">{{ s3.prefix || '—' }}</dd>
          </div>
          <div class="flex gap-3">
            <dt class="w-28 text-muted-foreground shrink-0">{{ t('import.s3_region') }}</dt>
            <dd class="font-mono">{{ s3.region || t('import.s3_region_default') }}</dd>
          </div>
          <div class="flex gap-3">
            <dt class="w-28 text-muted-foreground shrink-0">{{ t('import.s3_creds') }}</dt>
            <dd>{{ s3CredsLabel }}</dd>
          </div>
        </dl>
      </Card>
    </template>

    <!-- progress for exports only -->
    <div v-for="run in exportRuns" :key="run.id" class="space-y-2">
      <Card class="p-4">
        <div class="flex items-center gap-3 mb-2">
          <span class="font-medium">{{ t('export.export_label', { id: run.id.slice(0, 8) }) }}</span>
          <span class="text-xs px-1.5 py-0.5 rounded bg-accent">{{ run.kind }}</span>
          <span class="ml-auto text-sm text-muted-foreground">
            <span v-if="run.total">{{ run.processed }} / {{ run.total }}</span>
            <span v-if="run.duplicates"> · {{ run.duplicates }} {{ t('import.dup') }}</span>
            <span v-if="run.errors" class="text-destructive"> · {{ run.errors }} {{ t('import.err') }}</span>
          </span>
          <Button v-if="!run.done" size="sm" variant="outline" @click="cancelRun(run.id)">
            {{ t('import.abort') }}
          </Button>
        </div>

        <div class="flex items-center justify-between text-xs text-muted-foreground mb-1">
          <span :class="{ 'text-foreground': run.done && run.errors === 0 }">
            <span v-if="run.done && run.errors === 0">✓</span>
            <span v-else-if="run.done && run.errors > 0" class="text-destructive">✗</span>
            {{ run.phase }}
          </span>
          <span v-if="run.current" class="truncate ml-2 max-w-[50%]" :title="run.current">
            {{ run.current }}
          </span>
        </div>

        <div
          class="h-1.5 bg-muted rounded overflow-hidden"
          role="progressbar"
          :aria-label="t('export.export_label', { id: run.id.slice(0, 8) })"
          :aria-valuemin="0"
          :aria-valuemax="run.total || undefined"
          :aria-valuenow="run.total ? run.processed : undefined"
        >
          <div
            class="h-full bg-primary transition-all"
            :class="{ 'animate-pulse': !run.done && run.total === 0 }"
            :style="{
              width: run.total
                ? `${Math.min(100, (run.processed / run.total) * 100)}%`
                : (run.done ? '100%' : '15%'),
            }"
          ></div>
        </div>

        <details class="mt-3" v-if="run.log.length">
          <summary class="text-xs text-muted-foreground cursor-pointer">{{ t('import.log') }} ({{ run.log.length }})</summary>
          <ul class="mt-2 text-xs font-mono space-y-0.5 max-h-48 overflow-auto">
            <li v-for="(e, i) in run.log" :key="i" :class="{
              'text-destructive': e.kind === 'err',
              'text-muted-foreground': e.kind === 'dup',
            }">
              <span class="inline-block w-12">{{ e.kind }}</span>
              <span>{{ e.path }}</span>
              <span v-if="e.detail" class="ml-2 text-muted-foreground">{{ e.detail.slice(0, 80) }}</span>
            </li>
          </ul>
        </details>
      </Card>
    </div>
  </div>
</template>
