<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { TEXT_SIZES, textSize, type TextSize } from '@/lib/textSize'
import Card from '@/components/ui/Card.vue'
import { cn } from '@/lib/utils'

const { t } = useI18n()

// Each option's "Aa" is drawn at its own size relative to the current one, so
// the row reads as a scale wherever it is set.
const sample: Record<TextSize, string> = {
  xs: 'text-xs',
  sm: 'text-sm',
  md: 'text-base',
  lg: 'text-lg',
  xl: 'text-xl',
}
</script>

<template>
  <Card class="p-6 space-y-6">
    <h2 class="text-lg font-semibold">{{ t('settings.section.appearance') }}</h2>

    <div>
      <h3 id="text-size-label" class="text-sm font-medium mb-1">{{ t('appearance.text_size') }}</h3>
      <p class="text-sm text-muted-foreground mb-3">{{ t('appearance.text_size_help') }}</p>
      <div
        role="radiogroup"
        aria-labelledby="text-size-label"
        class="inline-flex flex-wrap gap-2"
      >
        <button
          v-for="s in TEXT_SIZES"
          :key="s"
          type="button"
          role="radio"
          :aria-checked="textSize === s"
          :class="cn(
            'flex w-20 flex-col items-center justify-end gap-1 rounded-md border px-3 py-2',
            'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
            textSize === s
              ? 'border-primary bg-primary/10 text-primary'
              : 'bg-card text-foreground hover:bg-muted',
          )"
          @click="textSize = s"
        >
          <span :class="cn('font-semibold leading-none', sample[s])" aria-hidden="true">Aa</span>
          <span class="text-xs">{{ t(`appearance.size.${s}`) }}</span>
        </button>
      </div>
      <p class="mt-4 rounded-md border bg-muted/50 p-4 text-sm">
        {{ t('appearance.preview') }}
      </p>
    </div>
  </Card>
</template>
