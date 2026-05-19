<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api, type Tag } from '@/lib/api'
import Card from '@/components/ui/Card.vue'

const { t } = useI18n()
const tags = ref<Tag[]>([])
const tagsLoading = ref(true)
const origin = window.location.origin

onMounted(async () => {
  try { tags.value = await api.listTags() } finally { tagsLoading.value = false }
})
</script>

<template>
  <div class="grid gap-6 md:grid-cols-2">
    <Card class="p-5">
      <h2 class="font-semibold mb-3">{{ t('settings.about') }}</h2>
      <dl class="text-sm space-y-1">
        <div class="flex justify-between"><dt class="text-muted-foreground">{{ t('settings.server') }}</dt><dd><code>{{ origin }}</code></dd></div>
        <div class="flex justify-between"><dt class="text-muted-foreground">{{ t('settings.build') }}</dt><dd>dev</dd></div>
      </dl>
      <p class="mt-3 text-xs text-muted-foreground">
        {{ t('settings.data_location', { path: '~/.local-eml/' }) }}
      </p>
    </Card>

    <Card class="p-5">
      <h2 class="font-semibold mb-3">{{ t('settings.tags') }}</h2>
      <p v-if="tagsLoading" class="text-sm text-muted-foreground">{{ t('settings.loading') }}</p>
      <p v-else-if="tags.length === 0" class="text-sm text-muted-foreground">{{ t('settings.no_tags') }}</p>
      <ul v-else class="text-sm divide-y">
        <li v-for="tg in tags" :key="tg.name" class="py-1.5 flex justify-between">
          <span>{{ tg.name }}</span>
          <span class="text-muted-foreground">{{ tg.count }}</span>
        </li>
      </ul>
    </Card>
  </div>
</template>
