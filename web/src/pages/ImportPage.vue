<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { cn } from '@/lib/utils'
import { formatBytes } from '@/lib/format'
import { useImports } from '@/composables/useImports'
import type { S3ImportConfig, IMAPImportConfig } from '@/lib/api'
import Card from '@/components/ui/Card.vue'
import Button from '@/components/ui/Button.vue'

const { t } = useI18n()
const { runs, startImport, startS3Import, startImapImport } = useImports()

type Provider = 'local' | 's3' | 'imap'
const provider = ref<Provider>('local')

// --- local upload (unchanged behavior) ---
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

// --- S3 provider ---
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
  await startS3Import(cfg)
}

function providerBtnClass(p: Provider) {
  return cn(
    'px-4 py-1.5 text-sm rounded-md cursor-pointer transition-colors',
    provider.value === p
      ? 'bg-primary text-primary-foreground'
      : 'text-muted-foreground hover:text-foreground',
  )
}

const inputClass =
  'w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-primary'

// Suggestions only — the input still accepts any value, so newly launched
// regions keep working without a code change.
const awsRegions: { code: string; name: string }[] = [
  { code: 'us-east-1', name: 'US East (N. Virginia)' },
  { code: 'us-east-2', name: 'US East (Ohio)' },
  { code: 'us-west-1', name: 'US West (N. California)' },
  { code: 'us-west-2', name: 'US West (Oregon)' },
  { code: 'ca-central-1', name: 'Canada (Central)' },
  { code: 'sa-east-1', name: 'South America (São Paulo)' },
  { code: 'eu-west-1', name: 'Europe (Ireland)' },
  { code: 'eu-west-2', name: 'Europe (London)' },
  { code: 'eu-west-3', name: 'Europe (Paris)' },
  { code: 'eu-central-1', name: 'Europe (Frankfurt)' },
  { code: 'eu-north-1', name: 'Europe (Stockholm)' },
  { code: 'eu-south-1', name: 'Europe (Milan)' },
  { code: 'ap-northeast-1', name: 'Asia Pacific (Tokyo)' },
  { code: 'ap-northeast-2', name: 'Asia Pacific (Seoul)' },
  { code: 'ap-northeast-3', name: 'Asia Pacific (Osaka)' },
  { code: 'ap-southeast-1', name: 'Asia Pacific (Singapore)' },
  { code: 'ap-southeast-2', name: 'Asia Pacific (Sydney)' },
  { code: 'ap-south-1', name: 'Asia Pacific (Mumbai)' },
  { code: 'ap-east-1', name: 'Asia Pacific (Hong Kong)' },
  { code: 'me-south-1', name: 'Middle East (Bahrain)' },
  { code: 'af-south-1', name: 'Africa (Cape Town)' },
]

// --- IMAP provider ---
const imapForm = ref<IMAPImportConfig>({
  host: '',
  port: 993,
  username: '',
  password: '',
  folder: 'INBOX',
})
const imapConfirming = ref(false)

const imapValid = computed(
  () =>
    imapForm.value.host.trim().length > 0 &&
    imapForm.value.username.trim().length > 0 &&
    imapForm.value.password.length > 0,
)

function reviewImap() {
  if (imapValid.value) imapConfirming.value = true
}

function cancelImap() {
  imapConfirming.value = false
}

async function confirmImap() {
  const cfg: IMAPImportConfig = {
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
      <button :class="providerBtnClass('local')" @click="provider = 'local'">
        {{ t('import.provider_local') }}
      </button>
      <button :class="providerBtnClass('s3')" @click="provider = 's3'">
        {{ t('import.provider_s3') }}
      </button>
      <button :class="providerBtnClass('imap')" @click="provider = 'imap'">
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
          <input ref="fileInput" type="file" multiple accept=".eml,.zip" class="hidden" @change="onFilePicked" />
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
            <Button @click="confirmUpload">{{ t('import.confirm') }}</Button>
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
    <template v-else-if="provider === 's3'">
      <Card v-if="!s3Confirming" class="p-6 space-y-4">
        <div>
          <h3 class="text-lg font-semibold mb-1">{{ t('import.s3_title') }}</h3>
          <p class="text-sm text-muted-foreground">{{ t('import.s3_creds_hint') }}</p>
        </div>

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
            <input
              v-model="s3.region"
              :class="inputClass"
              list="aws-regions"
              placeholder="us-east-1"
              autocomplete="off"
            />
            <datalist id="aws-regions">
              <option v-for="r in awsRegions" :key="r.code" :value="r.code">{{ r.name }}</option>
            </datalist>
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
          <Button :disabled="!s3Valid" @click="reviewS3">{{ t('import.s3_review') }}</Button>
        </div>
      </Card>

      <Card v-else class="p-6">
        <div class="flex items-start justify-between gap-4 mb-4">
          <div>
            <h3 class="text-lg font-semibold mb-1">{{ t('import.confirm_title') }}</h3>
            <p class="text-sm text-muted-foreground">{{ t('import.s3_confirm_hint') }}</p>
          </div>
          <div class="flex gap-2 shrink-0">
            <Button variant="outline" @click="cancelS3">{{ t('import.cancel') }}</Button>
            <Button @click="confirmS3">{{ t('import.confirm') }}</Button>
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

    <!-- IMAP provider -->
    <template v-else>
      <Card v-if="!imapConfirming" class="p-6 space-y-4">
        <div>
          <h3 class="text-lg font-semibold mb-1">{{ t('import.imap_title') }}</h3>
          <p class="text-sm text-muted-foreground">{{ t('import.imap_hint') }}</p>
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.imap_host') }} *</span>
            <input v-model="imapForm.host" :class="inputClass" placeholder="imap.example.com" autocomplete="off" />
          </label>
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.imap_port') }}</span>
            <input v-model.number="imapForm.port" type="number" :class="inputClass" placeholder="993" autocomplete="off" />
          </label>
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.imap_username') }} *</span>
            <input v-model="imapForm.username" :class="inputClass" autocomplete="off" />
          </label>
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.imap_password') }} *</span>
            <input v-model="imapForm.password" type="password" :class="inputClass" autocomplete="off" />
          </label>
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.imap_folder') }}</span>
            <input v-model="imapForm.folder" :class="inputClass" placeholder="INBOX" autocomplete="off" />
          </label>
        </div>

        <div class="flex justify-end">
          <Button :disabled="!imapValid" @click="reviewImap">{{ t('import.s3_review') }}</Button>
        </div>
      </Card>

      <Card v-else class="p-6">
        <div class="flex items-start justify-between gap-4 mb-4">
          <div>
            <h3 class="text-lg font-semibold mb-1">{{ t('import.confirm_title') }}</h3>
            <p class="text-sm text-muted-foreground">{{ t('import.imap_confirm_hint') }}</p>
          </div>
          <div class="flex gap-2 shrink-0">
            <Button variant="outline" @click="cancelImap">{{ t('import.cancel') }}</Button>
            <Button @click="confirmImap">{{ t('import.confirm') }}</Button>
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
            <dd class="font-mono">{{ imapForm.folder || 'INBOX' }}</dd>
          </div>
          <div class="flex gap-3">
            <dt class="w-28 text-muted-foreground shrink-0">{{ t('import.imap_port') }}</dt>
            <dd class="font-mono">{{ imapForm.port || 993 }}</dd>
          </div>
        </dl>
      </Card>
    </template>

    <!-- progress (shared) -->
    <div v-for="run in runs" :key="run.id" class="space-y-2">
      <Card class="p-4">
        <div class="flex items-center gap-3 mb-2">
          <span class="font-medium">{{ t('import.import_label', { id: run.id.slice(0, 8) }) }}</span>
          <span class="text-xs px-1.5 py-0.5 rounded bg-accent">{{ run.kind }}</span>
          <span class="ml-auto text-sm text-muted-foreground">
            <span v-if="run.total">{{ run.processed }} / {{ run.total }}</span>
            <span v-if="run.duplicates"> · {{ run.duplicates }} {{ t('import.dup') }}</span>
            <span v-if="run.errors" class="text-destructive"> · {{ run.errors }} {{ t('import.err') }}</span>
          </span>
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

        <div class="h-1.5 bg-muted rounded overflow-hidden">
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
