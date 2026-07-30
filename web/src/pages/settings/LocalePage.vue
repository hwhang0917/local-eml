<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { setLocale, type Locale } from '@/i18n'
import { dateFormat, weekStartsOn, type DateFormat, type WeekStart } from '@/lib/format'
import Card from '@/components/ui/Card.vue'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select'

const { t, locale } = useI18n()

const langs: { value: Locale; label: string }[] = [
  { value: 'en', label: 'English' },
  { value: 'ko', label: '한국어' },
]
const currentLangLabel = computed(
  () => langs.find((l) => l.value === locale.value)?.label ?? locale.value,
)

const dateFormats = computed<{ value: DateFormat; label: string }[]>(() => [
  { value: 'absolute', label: t('settings.date_format_absolute') },
  { value: 'relative', label: t('settings.date_format_relative') },
])
const currentDateFormatLabel = computed(
  () => dateFormats.value.find((d) => d.value === dateFormat.value)?.label ?? dateFormat.value,
)

const weekStarts = computed<{ value: WeekStart; label: string }[]>(() => [
  { value: 0, label: t('settings.week_start_sunday') },
  { value: 1, label: t('settings.week_start_monday') },
])
const currentWeekStartLabel = computed(
  () => weekStarts.value.find((w) => w.value === weekStartsOn.value)?.label ?? '',
)
</script>

<template>
  <Card class="p-6 space-y-6">
    <h2 class="text-lg font-semibold">{{ t('settings.section.locale') }}</h2>

    <div>
      <h3 class="text-sm font-medium mb-1">{{ t('settings.language') }}</h3>
      <p class="text-sm text-muted-foreground mb-3">{{ t('settings.language_help') }}</p>
      <Select
        :model-value="locale"
        @update:model-value="(v) => v && setLocale(v as Locale)"
      >
        <SelectTrigger class="w-72">
          <SelectValue>{{ currentLangLabel }}</SelectValue>
        </SelectTrigger>
        <SelectContent>
          <SelectItem v-for="l in langs" :key="l.value" :value="l.value">
            {{ l.label }}
          </SelectItem>
        </SelectContent>
      </Select>
    </div>

    <div>
      <h3 class="text-sm font-medium mb-1">{{ t('settings.date_format') }}</h3>
      <p class="text-sm text-muted-foreground mb-3">{{ t('settings.date_format_help') }}</p>
      <Select
        :model-value="dateFormat"
        @update:model-value="(v) => v && (dateFormat = v as DateFormat)"
      >
        <SelectTrigger class="w-full max-w-md whitespace-nowrap">
          <SelectValue>{{ currentDateFormatLabel }}</SelectValue>
        </SelectTrigger>
        <SelectContent>
          <SelectItem v-for="d in dateFormats" :key="d.value" :value="d.value">
            {{ d.label }}
          </SelectItem>
        </SelectContent>
      </Select>
    </div>

    <div>
      <h3 class="text-sm font-medium mb-1">{{ t('settings.week_start') }}</h3>
      <p class="text-sm text-muted-foreground mb-3">{{ t('settings.week_start_help') }}</p>
      <Select
        :model-value="String(weekStartsOn)"
        @update:model-value="(v) => v != null && (weekStartsOn = Number(v) as WeekStart)"
      >
        <SelectTrigger class="w-72">
          <SelectValue>{{ currentWeekStartLabel }}</SelectValue>
        </SelectTrigger>
        <SelectContent>
          <SelectItem v-for="w in weekStarts" :key="w.value" :value="String(w.value)">
            {{ w.label }}
          </SelectItem>
        </SelectContent>
      </Select>
    </div>
  </Card>
</template>
