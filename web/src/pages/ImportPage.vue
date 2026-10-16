<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { cn, fieldClass, segmentClass } from '@/lib/utils'
import { formatBytes, formatEpoch } from '@/lib/format'
import { useImports } from '@/composables/useImports'
import { NEW_PROFILE } from '@/composables/useS3Form'
import { toast } from 'vue-sonner'
import { api } from '@/lib/api'
import type { IMAPImportConfig, IMAPProfile } from '@/lib/api'
import Card from '@/components/ui/Card.vue'
import Button from '@/components/ui/Button.vue'
import ConfirmDialog from '@/components/ui/ConfirmDialog.vue'
import S3FormCard from '@/components/S3FormCard.vue'
import JobRunCard from '@/components/JobRunCard.vue'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select'

const IMAP_DEFAULT_PORT = 993
const IMAP_DEFAULT_FOLDER = 'INBOX'

// Seconds. Mirrors what the server accepts; 0 turns background sync off.
const SYNC_INTERVALS_S = [0, 300, 600, 1800, 3600]
// Also the server default, so the help text never renders blank before the
// fetch corrects it.
const DEFAULT_SYNC_INTERVAL_S = 600

const { t } = useI18n()
const { importRuns, importActive, startImport, startS3Import, startImapImport } = useImports()

type Provider = 'local' | 's3' | 'imap'
const provider = ref<Provider>('local')

// --- local upload ---
const stagedFiles = ref<File[]>([])
const dragOver = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
const dirInput = ref<HTMLInputElement | null>(null)

const stagedTotal = computed(() =>
  stagedFiles.value.reduce((s, f) => s + f.size, 0),
)

const stagedSummary = computed(() => {
  const total = formatBytes(stagedTotal.value)
  return stagedFiles.value.length === 1
    ? t('import.confirm_summary_one', { total })
    : t('import.confirm_summary_many', { count: stagedFiles.value.length, total })
})

function stageFiles(files: File[]) {
  if (files.length === 0) return
  stagedFiles.value = files
}

function clearStaged() {
  stagedFiles.value = []
}

async function confirmUpload() {
  if (stagedFiles.value.length === 0) return
  const files = stagedFiles.value
  stagedFiles.value = []
  await startImport(files)
}

function onDrop(e: DragEvent) {
  e.preventDefault()
  dragOver.value = false
  const files = e.dataTransfer?.files
  if (!files) return
  stageFiles(Array.from(files))
}

function onFilePicked(e: Event) {
  const input = e.target as HTMLInputElement
  if (!input.files) return
  stageFiles(Array.from(input.files))
  input.value = ''
}

const dropzoneClass = computed(() =>
  cn(
    'border-2 border-dashed text-center p-12 transition-colors',
    dragOver.value ? 'border-primary bg-accent/50' : 'border-border',
  ),
)

// --- IMAP provider ---
function emptyImap(): IMAPImportConfig {
  return { host: '', port: IMAP_DEFAULT_PORT, username: '', password: '', folder: IMAP_DEFAULT_FOLDER }
}

const imapForm = ref<IMAPImportConfig>(emptyImap())
const imapConfirming = ref(false)
const imapSyncEnabled = ref(false)
const imapSyncing = ref(false)

const imapValid = computed(
  () =>
    imapForm.value.host.trim().length > 0 &&
    imapForm.value.username.trim().length > 0 &&
    imapForm.value.password.length > 0,
)

// --- IMAP profiles ---
const imapProfiles = ref<IMAPProfile[]>([])
const selectedImapProfileId = ref<number | null>(null)
const imapDeleteConfirmOpen = ref(false)

const activeImapProfile = computed(() =>
  imapProfiles.value.find((p) => p.id === selectedImapProfileId.value) ?? null,
)

const canSaveImapProfile = computed(
  () =>
    imapForm.value.host.trim().length > 0 &&
    imapForm.value.username.trim().length > 0,
)

async function loadImapProfiles() {
  try {
    imapProfiles.value = await api.listIMAPProfiles()
  } catch (e) {
    toast.error(t('import.imap_profile_load_error'), { description: String(e) })
  }
}

// --- IMAP background poll interval ---
const syncIntervalSeconds = ref<number>(DEFAULT_SYNC_INTERVAL_S)

function intervalLabel(s: number): string {
  if (s === 0) return t('import.sync_interval_off')
  return s % 3600 === 0
    ? t('import.sync_interval_hours', { n: s / 3600 }, s / 3600)
    : t('import.sync_interval_minutes', { n: Math.round(s / 60) })
}

const syncIntervalChoices = computed(() =>
  SYNC_INTERVALS_S.map((value) => ({ value, label: intervalLabel(value) })),
)
// An interval set via env var to something off-menu is still stated exactly.
const syncIntervalLabel = computed(() => intervalLabel(syncIntervalSeconds.value))

async function loadSyncInterval() {
  try {
    syncIntervalSeconds.value = (await api.getIMAPSyncInterval()).seconds
  } catch {
    // control simply shows nothing selected; saving still surfaces its error
  }
}

async function setSyncInterval(v: string | undefined) {
  if (v == null) return
  const seconds = Number(v)
  const prev = syncIntervalSeconds.value
  syncIntervalSeconds.value = seconds
  try {
    await api.setIMAPSyncInterval(seconds)
  } catch (e) {
    syncIntervalSeconds.value = prev
    toast.error(t('import.sync_interval_error'), { description: String(e) })
  }
}

onMounted(() => {
  loadImapProfiles()
  loadSyncInterval()
})

function applyImapProfile(p: IMAPProfile | null) {
  if (!p) {
    imapForm.value = emptyImap()
    imapSyncEnabled.value = false
    return
  }
  imapForm.value = {
    host: p.host,
    port: p.port ?? IMAP_DEFAULT_PORT,
    username: p.username,
    password: '',
    folder: p.folder ?? IMAP_DEFAULT_FOLDER,
  }
  imapSyncEnabled.value = p.sync_enabled
}

function onImapProfileChange(value: string) {
  if (value === NEW_PROFILE) {
    selectedImapProfileId.value = null
    applyImapProfile(null)
    return
  }
  const id = Number(value)
  selectedImapProfileId.value = id
  applyImapProfile(imapProfiles.value.find((p) => p.id === id) ?? null)
}

const imapProfileSelectValue = computed(() =>
  selectedImapProfileId.value == null ? NEW_PROFILE : String(selectedImapProfileId.value),
)

const imapProfileSelectLabel = computed(() =>
  activeImapProfile.value?.name ?? t('import.imap_profile_new'),
)

async function saveImapProfile() {
  const existing = activeImapProfile.value
  let name = existing?.name ?? ''
  if (!existing) {
    const entered = window.prompt(t('import.imap_profile_save_prompt'), '')
    if (entered == null) return
    name = entered.trim()
    if (!name) return
  }
  try {
    const saved = await api.saveIMAPProfile({
      name,
      host: imapForm.value.host.trim(),
      port: imapForm.value.port || undefined,
      username: imapForm.value.username.trim(),
      folder: imapForm.value.folder?.trim() || undefined,
      sync_enabled: imapSyncEnabled.value,
    })
    await loadImapProfiles()
    selectedImapProfileId.value = saved.id
  } catch (e) {
    toast.error(t('import.imap_profile_save_error'), { description: String(e) })
  }
}

async function syncImapNow() {
  const p = activeImapProfile.value
  if (!p) return
  imapSyncing.value = true
  try {
    await api.syncIMAPProfile(p.id)
    toast.success(t('import.imap_sync_started'))
  } catch (e) {
    toast.error(t('import.imap_sync_error'), { description: String(e) })
  } finally {
    imapSyncing.value = false
    await loadImapProfiles()
  }
}

async function deleteImapProfile() {
  const p = activeImapProfile.value
  if (!p) return
  try {
    await api.deleteIMAPProfile(p.id)
    selectedImapProfileId.value = null
    applyImapProfile(null)
    await loadImapProfiles()
  } catch (e) {
    toast.error(t('import.imap_profile_delete_error'), { description: String(e) })
  }
}

function reviewImap() {
  if (imapValid.value) imapConfirming.value = true
}

function cancelImap() {
  imapConfirming.value = false
}

async function confirmImap() {
  const cfg: IMAPImportConfig = {
    profile_id: selectedImapProfileId.value ?? undefined,
    host: imapForm.value.host.trim(),
    username: imapForm.value.username.trim(),
    password: imapForm.value.password,
    port: imapForm.value.port || undefined,
    folder: imapForm.value.folder?.trim() || undefined,
  }
  imapConfirming.value = false
  await startImapImport(cfg)
}
</script>

<template>
  <div class="space-y-6">
    <div class="inline-flex gap-1 p-1 rounded-lg bg-muted">
      <button :class="segmentClass(provider === 'local')" :aria-pressed="provider === 'local'" @click="provider = 'local'">
        {{ t('import.provider_local') }}
      </button>
      <button :class="segmentClass(provider === 's3')" :aria-pressed="provider === 's3'" @click="provider = 's3'">
        {{ t('import.provider_s3') }}
      </button>
      <button :class="segmentClass(provider === 'imap')" :aria-pressed="provider === 'imap'" @click="provider = 'imap'">
        {{ t('import.provider_imap') }}
      </button>
    </div>

    <!-- LOCAL provider -->
    <template v-if="provider === 'local'">
      <Card
        v-if="stagedFiles.length === 0"
        :class="dropzoneClass"
        @dragover.prevent="dragOver = true"
        @dragenter.prevent="dragOver = true"
        @dragleave="dragOver = false"
        @drop="onDrop"
      >
        <p class="text-lg mb-2">{{ t('import.drop') }}</p>
        <p class="text-sm text-muted-foreground mb-4">{{ t('import.dedup_note') }}</p>
        <div class="flex justify-center gap-2">
          <Button variant="outline" @click="fileInput?.click()">{{ t('import.choose_files') }}</Button>
          <Button variant="outline" @click="dirInput?.click()">{{ t('import.choose_folder') }}</Button>
          <input ref="fileInput" type="file" multiple accept=".eml,.zip,.mbox,.pst" class="hidden" @change="onFilePicked" />
          <input
            ref="dirInput"
            type="file"
            multiple
            class="hidden"
            @change="onFilePicked"
            v-bind="{ webkitdirectory: true, directory: true } as any"
          />
        </div>
      </Card>

      <Card v-else class="p-6">
        <div class="flex items-start justify-between gap-4 mb-4">
          <div>
            <h3 class="text-lg font-semibold mb-1">{{ t('import.confirm_title') }}</h3>
            <p class="text-sm text-muted-foreground">{{ stagedSummary }}</p>
          </div>
          <div class="flex gap-2 shrink-0">
            <Button variant="outline" @click="clearStaged">{{ t('import.cancel') }}</Button>
            <Button :disabled="importActive" @click="confirmUpload">{{ importActive ? t('import.busy') : t('import.confirm') }}</Button>
          </div>
        </div>
        <ul class="text-xs font-mono space-y-1 max-h-56 overflow-auto border border-hairline rounded-sm p-3 bg-muted/30">
          <li v-for="(f, i) in stagedFiles" :key="i" class="flex justify-between gap-3">
            <span class="truncate" :title="f.name">{{ f.name }}</span>
            <span class="text-muted-foreground shrink-0">{{ formatBytes(f.size) }}</span>
          </li>
        </ul>
      </Card>
    </template>

    <!-- S3 provider -->
    <S3FormCard
      v-else-if="provider === 's3'"
      title-key="import.s3_title"
      confirm-title-key="import.confirm_title"
      confirm-hint-key="import.s3_confirm_hint"
      start-key="import.confirm"
      busy-key="import.busy"
      :busy="importActive"
      :start="startS3Import"
    />

    <!-- IMAP provider -->
    <template v-else>
      <Card v-if="imapConfirming" class="p-6">
        <div class="flex items-start justify-between gap-4 mb-4">
          <div>
            <h3 class="text-lg font-semibold mb-1">{{ t('import.confirm_title') }}</h3>
            <p class="text-sm text-muted-foreground">{{ t('import.imap_confirm_hint') }}</p>
          </div>
          <div class="flex gap-2 shrink-0">
            <Button variant="outline" @click="cancelImap">{{ t('import.cancel') }}</Button>
            <Button :disabled="importActive" @click="confirmImap">{{ importActive ? t('import.busy') : t('import.confirm') }}</Button>
          </div>
        </div>
        <dl class="text-sm space-y-2">
          <div class="flex gap-3">
            <dt class="w-28 text-muted-foreground shrink-0">{{ t('import.imap_host') }}</dt>
            <dd class="font-mono">{{ imapForm.host }}</dd>
          </div>
          <div class="flex gap-3">
            <dt class="w-28 text-muted-foreground shrink-0">{{ t('import.imap_username') }}</dt>
            <dd class="font-mono">{{ imapForm.username }}</dd>
          </div>
          <div class="flex gap-3">
            <dt class="w-28 text-muted-foreground shrink-0">{{ t('import.imap_folder') }}</dt>
            <dd class="font-mono">{{ imapForm.folder || IMAP_DEFAULT_FOLDER }}</dd>
          </div>
          <div class="flex gap-3">
            <dt class="w-28 text-muted-foreground shrink-0">{{ t('import.imap_port') }}</dt>
            <dd class="font-mono">{{ imapForm.port || IMAP_DEFAULT_PORT }}</dd>
          </div>
        </dl>
      </Card>

      <Card v-else class="p-6 space-y-4">
        <div>
          <h3 class="text-lg font-semibold mb-1">{{ t('import.imap_title') }}</h3>
          <p class="text-sm text-muted-foreground">{{ t('import.imap_hint') }}</p>
        </div>

        <div class="flex items-end gap-2">
          <div class="space-y-1 flex-1 max-w-xs">
            <span class="text-sm">{{ t('import.imap_profile') }}</span>
            <Select
              :model-value="imapProfileSelectValue"
              @update:model-value="(v) => v && onImapProfileChange(String(v))"
            >
              <SelectTrigger class="w-full">
                <SelectValue>{{ imapProfileSelectLabel }}</SelectValue>
              </SelectTrigger>
              <SelectContent>
                <SelectItem :value="NEW_PROFILE">{{ t('import.imap_profile_new') }}</SelectItem>
                <SelectItem v-for="p in imapProfiles" :key="p.id" :value="String(p.id)">
                  {{ p.name }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <Button variant="outline" :disabled="!canSaveImapProfile" @click="saveImapProfile">
            {{ t('import.imap_profile_save') }}
          </Button>
          <Button variant="outline" :disabled="!activeImapProfile" @click="imapDeleteConfirmOpen = true">
            {{ t('import.imap_profile_delete') }}
          </Button>
          <ConfirmDialog
            v-model:open="imapDeleteConfirmOpen"
            :title="t('import.imap_profile_delete_confirm', { name: activeImapProfile?.name ?? '' })"
            :description="t('import.profile_delete_body')"
            :confirm-label="t('import.imap_profile_delete')"
            :cancel-label="t('import.cancel')"
            destructive
            @confirm="deleteImapProfile"
          />
        </div>

        <div class="rounded-md border border-border bg-muted/30 p-3 space-y-2">
          <label class="flex items-start gap-2 cursor-pointer">
            <input
              type="checkbox"
              class="mt-0.5"
              :checked="imapSyncEnabled"
              @change="imapSyncEnabled = ($event.target as HTMLInputElement).checked"
            />
            <span class="text-sm">
              <span class="font-medium">{{ t('import.imap_sync_label') }}</span>
              <span class="block text-xs text-muted-foreground mt-0.5">
                {{ syncIntervalSeconds === 0
                  ? t('import.imap_sync_help_off')
                  : t('import.imap_sync_help', { interval: syncIntervalLabel }) }}
              </span>
            </span>
          </label>
          <div class="flex items-center gap-2 pl-6">
            <span class="text-xs text-muted-foreground">{{ t('import.sync_interval_label') }}</span>
            <Select
              :model-value="String(syncIntervalSeconds)"
              @update:model-value="(v) => setSyncInterval(v as string)"
            >
              <SelectTrigger class="h-8 w-40 text-xs">
                <SelectValue>{{ syncIntervalLabel }}</SelectValue>
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="c in syncIntervalChoices" :key="c.value" :value="String(c.value)">
                  {{ c.label }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div
            v-if="activeImapProfile && activeImapProfile.sync_enabled"
            class="flex items-center gap-3 text-xs text-muted-foreground pl-6"
          >
            <span v-if="activeImapProfile.last_synced_at">
              {{ t('import.imap_last_synced', { when: formatEpoch(activeImapProfile.last_synced_at) }) }}
            </span>
            <span v-else>{{ t('import.imap_never_synced') }}</span>
            <Button
              v-if="activeImapProfile.has_password"
              size="sm"
              variant="outline"
              :disabled="imapSyncing"
              @click="syncImapNow"
            >
              {{ imapSyncing ? t('import.imap_sync_running') : t('import.imap_sync_now') }}
            </Button>
            <span v-else class="text-amber-600">
              · {{ t('import.imap_sync_needs_password') }}
            </span>
          </div>
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.imap_host') }} *</span>
            <input v-model="imapForm.host" :class="fieldClass" placeholder="imap.example.com" autocomplete="off" />
          </label>
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.imap_port') }}</span>
            <input v-model.number="imapForm.port" type="number" :class="fieldClass" :placeholder="String(IMAP_DEFAULT_PORT)" autocomplete="off" />
          </label>
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.imap_username') }} *</span>
            <input v-model="imapForm.username" :class="fieldClass" autocomplete="off" />
          </label>
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.imap_password') }} *</span>
            <input v-model="imapForm.password" type="password" :class="fieldClass" autocomplete="off" />
          </label>
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.imap_folder') }}</span>
            <input v-model="imapForm.folder" :class="fieldClass" :placeholder="IMAP_DEFAULT_FOLDER" autocomplete="off" />
          </label>
        </div>

        <div class="flex justify-end">
          <Button :disabled="!imapValid" @click="reviewImap">{{ t('import.review') }}</Button>
        </div>
      </Card>
    </template>

    <JobRunCard v-for="run in importRuns" :key="run.id" :run="run" label-key="import.import_label" />
  </div>
</template>
