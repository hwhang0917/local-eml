<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { segmentClass } from '@/lib/utils'
import { useImports } from '@/composables/useImports'
import { api } from '@/lib/api'
import Card from '@/components/ui/Card.vue'
import Button from '@/components/ui/Button.vue'
import S3FormCard from '@/components/S3FormCard.vue'
import JobRunCard from '@/components/JobRunCard.vue'

const { t } = useI18n()
const { exportRuns, exportActive, startS3Export } = useImports()

type Mode = 'zip' | 's3'
const mode = ref<Mode>('zip')

function downloadZip() {
  window.location.assign(api.exportZipURL())
}
</script>

<template>
  <div class="space-y-6">
    <div class="inline-flex gap-1 p-1 rounded-lg bg-muted">
      <button :class="segmentClass(mode === 'zip')" :aria-pressed="mode === 'zip'" @click="mode = 'zip'">
        {{ t('export.mode_zip') }}
      </button>
      <button :class="segmentClass(mode === 's3')" :aria-pressed="mode === 's3'" @click="mode = 's3'">
        {{ t('export.mode_s3') }}
      </button>
    </div>

    <Card v-if="mode === 'zip'" class="p-6 space-y-4">
      <div>
        <h3 class="text-lg font-semibold mb-1">{{ t('export.zip_title') }}</h3>
        <p class="text-sm text-muted-foreground">{{ t('export.zip_hint') }}</p>
      </div>
      <div class="flex justify-end">
        <Button @click="downloadZip">{{ t('export.zip_download') }}</Button>
      </div>
    </Card>

    <S3FormCard
      v-else
      title-key="export.s3_title"
      hint-key="export.s3_hint"
      confirm-title-key="export.confirm_title"
      confirm-hint-key="export.s3_confirm_hint"
      start-key="export.start"
      busy-key="export.busy"
      :busy="exportActive"
      :start="startS3Export"
    />

    <JobRunCard v-for="run in exportRuns" :key="run.id" :run="run" label-key="export.export_label" />
  </div>
</template>
