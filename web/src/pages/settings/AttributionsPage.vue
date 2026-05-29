<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { attributions } from '@/lib/attributions'
import Card from '@/components/ui/Card.vue'

const { t } = useI18n()
</script>

<template>
  <Card class="p-6 space-y-4">
    <h2 class="text-lg font-semibold">{{ t('settings.section.attributions') }}</h2>
    <p class="text-sm text-muted-foreground">{{ t('settings.attributions_intro') }}</p>
    <ul class="text-sm divide-y">
      <li
        v-for="a in attributions"
        :key="a.name"
        class="py-2 flex items-center justify-between gap-3"
      >
        <a
          :href="a.url"
          target="_blank"
          rel="noopener noreferrer"
          class="text-primary hover:underline"
        >{{ a.name }}</a>
        <span class="text-xs text-muted-foreground">{{ a.license }}</span>
      </li>
    </ul>
  </Card>
</template>
