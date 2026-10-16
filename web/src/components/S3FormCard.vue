<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { fieldClass } from '@/lib/utils'
import type { S3ImportConfig } from '@/lib/api'
import { useS3Form, NEW_PROFILE } from '@/composables/useS3Form'
import Card from '@/components/ui/Card.vue'
import Button from '@/components/ui/Button.vue'
import ConfirmDialog from '@/components/ui/ConfirmDialog.vue'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select'

// i18n keys rather than strings, so the page stays the only place that knows
// which namespace it speaks.
const props = defineProps<{
  titleKey: string
  hintKey?: string
  confirmTitleKey: string
  confirmHintKey: string
  startKey: string
  busyKey: string
  busy: boolean
  start: (cfg: S3ImportConfig) => Promise<void>
}>()

const { t } = useI18n()
const {
  s3, confirming, valid, credsLabel,
  profiles, activeProfile, profileSelectValue, profileSelectLabel, deleteConfirmOpen,
  onProfileChange, saveProfile, deleteProfile, review, cancel, confirm: confirmS3,
} = useS3Form()

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
</script>

<template>
  <Card v-if="confirming" class="p-6">
    <div class="flex items-start justify-between gap-4 mb-4">
      <div>
        <h3 class="text-lg font-semibold mb-1">{{ t(confirmTitleKey) }}</h3>
        <p class="text-sm text-muted-foreground">{{ t(confirmHintKey) }}</p>
      </div>
      <div class="flex gap-2 shrink-0">
        <Button variant="outline" @click="cancel">{{ t('import.cancel') }}</Button>
        <Button :disabled="busy" @click="confirmS3(props.start)">{{ busy ? t(busyKey) : t(startKey) }}</Button>
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
        <dd>{{ credsLabel }}</dd>
      </div>
    </dl>
  </Card>

  <Card v-else class="p-6 space-y-4">
    <div>
      <h3 class="text-lg font-semibold mb-1">{{ t(titleKey) }}</h3>
      <p v-if="hintKey" class="text-sm text-muted-foreground">{{ t(hintKey) }}</p>
    </div>

    <div class="flex items-end gap-2">
      <div class="space-y-1 flex-1 max-w-xs">
        <span class="text-sm">{{ t('import.s3_profile') }}</span>
        <Select
          :model-value="profileSelectValue"
          @update:model-value="(v) => v && onProfileChange(String(v))"
        >
          <SelectTrigger class="w-full">
            <SelectValue>{{ profileSelectLabel }}</SelectValue>
          </SelectTrigger>
          <SelectContent>
            <SelectItem :value="NEW_PROFILE">{{ t('import.s3_profile_new') }}</SelectItem>
            <SelectItem v-for="p in profiles" :key="p.id" :value="String(p.id)">
              {{ p.name }}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>
      <Button variant="outline" :disabled="!valid" @click="saveProfile">
        {{ t('import.s3_profile_save') }}
      </Button>
      <Button variant="outline" :disabled="!activeProfile" @click="deleteConfirmOpen = true">
        {{ t('import.s3_profile_delete') }}
      </Button>
      <ConfirmDialog
        v-model:open="deleteConfirmOpen"
        :title="t('import.s3_profile_delete_confirm', { name: activeProfile?.name ?? '' })"
        :description="t('import.profile_delete_body')"
        :confirm-label="t('import.s3_profile_delete')"
        :cancel-label="t('import.cancel')"
        destructive
        @confirm="deleteProfile"
      />
    </div>

    <p class="text-sm text-muted-foreground">{{ t('import.s3_creds_optional') }}</p>
    <div class="grid gap-4 sm:grid-cols-2">
      <label class="space-y-1">
        <span class="text-sm">{{ t('import.s3_access_key') }}</span>
        <input v-model="s3.accessKeyId" :class="fieldClass" autocomplete="off" />
      </label>
      <label class="space-y-1">
        <span class="text-sm">{{ t('import.s3_secret_key') }}</span>
        <input v-model="s3.secretAccessKey" type="password" :class="fieldClass" autocomplete="off" />
      </label>
      <label class="space-y-1">
        <span class="text-sm">{{ t('import.s3_session_token') }}</span>
        <input v-model="s3.sessionToken" type="password" :class="fieldClass" autocomplete="off" />
      </label>
      <label class="space-y-1">
        <span class="text-sm">{{ t('import.s3_region') }}</span>
        <input
          v-model="s3.region"
          :class="fieldClass"
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
        <input v-model="s3.bucket" :class="fieldClass" autocomplete="off" />
      </label>
      <label class="space-y-1">
        <span class="text-sm">{{ t('import.s3_prefix') }}</span>
        <input v-model="s3.prefix" :class="fieldClass" placeholder="mail/2026/" autocomplete="off" />
      </label>
    </div>

    <div class="flex justify-end">
      <Button :disabled="!valid" @click="review">{{ t('import.review') }}</Button>
    </div>
  </Card>
</template>
