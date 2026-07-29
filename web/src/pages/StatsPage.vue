<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api, type Stats } from '@/lib/api'
import { formatBytes, senderName } from '@/lib/format'
import { useCategories } from '@/composables/useCategories'
import Card from '@/components/ui/Card.vue'
import CategoryDot from '@/components/ui/CategoryDot.vue'

const { t, locale } = useI18n()
const n = (v: number) => v.toLocaleString(locale.value)
const { byId, labelFor, load: loadCategories } = useCategories()

const stats = ref<Stats | null>(null)
const error = ref('')

onMounted(async () => {
  try {
    ;[stats.value] = await Promise.all([api.getStats(), loadCategories()])
  } catch (e) {
    error.value = String(e)
  }
})

const tiles = computed(() => {
  const s = stats.value
  if (!s) return []
  return [
    { label: t('stats.total'), value: n(s.total_count) },
    { label: t('stats.total_size'), value: formatBytes(s.total_bytes) },
    { label: t('stats.starred'), value: n(s.starred_count) },
    { label: t('stats.with_attachments'), value: n(s.attachment_count) },
  ]
})

const maxYear = computed(() =>
  Math.max(1, ...(stats.value?.per_year.map((y) => y.count) ?? [])),
)

const categoryRows = computed(() =>
  (stats.value?.per_category ?? [])
    .map((c) => ({ ...c, category: byId.value.get(c.category_id) }))
    .filter((c) => c.category)
    .sort((a, b) => b.count - a.count),
)
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-semibold tracking-tight">{{ t('stats.title') }}</h1>

    <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
    <p v-else-if="stats && stats.total_count === 0" class="text-sm text-muted-foreground">
      {{ t('stats.empty') }}
    </p>

    <template v-else-if="stats">
      <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <Card v-for="tile in tiles" :key="tile.label" class="p-4">
          <div class="text-sm text-muted-foreground">{{ tile.label }}</div>
          <div class="mt-1 text-2xl font-semibold tabular-nums">{{ tile.value }}</div>
        </Card>
      </div>

      <div class="grid gap-6 lg:grid-cols-2">
        <Card class="p-6">
          <h2 class="text-base font-semibold">{{ t('stats.per_year') }}</h2>
          <ul class="mt-4 space-y-2">
            <li
              v-for="y in stats.per_year"
              :key="y.year"
              class="flex items-center gap-3 text-sm"
            >
              <span class="w-12 shrink-0 tabular-nums text-muted-foreground">{{ y.year }}</span>
              <span class="h-4 min-w-0.5 rounded-r-sm bg-primary" :style="{ width: `${(y.count / maxYear) * 88}%` }"></span>
              <span class="tabular-nums">{{ n(y.count) }}</span>
            </li>
          </ul>
          <p v-if="stats.undated_count > 0" class="mt-4 text-xs text-muted-foreground">
            {{ t('stats.undated', { count: n(stats.undated_count) }) }}
          </p>
        </Card>

        <div class="space-y-6">
          <Card class="p-6">
            <h2 class="text-base font-semibold">{{ t('stats.top_senders') }}</h2>
            <ol class="mt-4 space-y-2">
              <li
                v-for="s in stats.top_senders"
                :key="s.from"
                class="flex items-baseline gap-3 text-sm"
              >
                <span class="min-w-0 flex-1 truncate" :title="s.from">{{ senderName(s.from) || s.from }}</span>
                <span class="shrink-0 tabular-nums text-muted-foreground">{{ n(s.count) }}</span>
              </li>
            </ol>
          </Card>

          <Card v-if="categoryRows.length" class="p-6">
            <h2 class="text-base font-semibold">{{ t('stats.per_category') }}</h2>
            <ul class="mt-4 space-y-2">
              <li
                v-for="c in categoryRows"
                :key="c.category_id"
                class="flex items-center gap-2 text-sm"
              >
                <CategoryDot :color="c.category!.color" />
                <span class="min-w-0 flex-1 truncate">{{ labelFor(c.category) }}</span>
                <span class="shrink-0 tabular-nums text-muted-foreground">{{ n(c.count) }}</span>
              </li>
            </ul>
          </Card>
        </div>
      </div>
    </template>
  </div>
</template>
