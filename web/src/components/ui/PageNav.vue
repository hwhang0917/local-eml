<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Button from '@/components/ui/Button.vue'

defineProps<{ info: string; canPrev: boolean; canNext: boolean }>()
const emit = defineEmits<{ prev: []; next: [] }>()
const { t } = useI18n()
</script>

<template>
  <nav :aria-label="t('library.pagination')" class="flex items-center gap-3">
    <span class="text-sm text-muted-foreground">{{ info }}</span>
    <div class="flex items-center gap-1">
      <Button variant="outline" size="sm" :disabled="!canPrev" @click="emit('prev')">
        {{ t('library.prev') }}
      </Button>
      <Button variant="outline" size="sm" :disabled="!canNext" @click="emit('next')">
        {{ t('library.next') }}
      </Button>
    </div>
  </nav>
</template>
