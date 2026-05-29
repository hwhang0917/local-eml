<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Github } from 'lucide-vue-next'
import { APP_VERSION } from '@/version'
import Card from '@/components/ui/Card.vue'

const { t } = useI18n()
const origin = window.location.origin
const REPO_URL = 'https://github.com/hwhang0917/local-eml'
</script>

<template>
  <Card class="p-6 space-y-4">
    <h2 class="text-lg font-semibold">{{ t('settings.section.about') }}</h2>
    <dl class="text-sm grid grid-cols-[10rem_1fr] gap-y-2">
      <dt class="text-muted-foreground">{{ t('settings.version') }}</dt>
      <dd>v{{ APP_VERSION }}</dd>
      <dt class="text-muted-foreground">{{ t('settings.server') }}</dt>
      <dd><code>{{ origin }}</code></dd>
    </dl>
    <p class="text-xs text-muted-foreground">
      {{ t('settings.data_location', { path: '~/.local-eml/' }) }}
    </p>
    <div>
      <a
        :href="REPO_URL"
        target="_blank"
        rel="noopener noreferrer"
        class="inline-flex items-center gap-2 text-sm text-primary hover:underline"
      >
        <Github class="h-4 w-4" />
        {{ t('settings.github') }}
      </a>
    </div>
  </Card>
</template>
