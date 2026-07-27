<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { api, type Category } from '@/lib/api'
import { useCategories } from '@/composables/useCategories'
import Card from '@/components/ui/Card.vue'
import CategoryDot from '@/components/ui/CategoryDot.vue'

const { t } = useI18n()
const { categories, labelFor, reload } = useCategories()

const busy = ref(false)

onMounted(reload)

// The set is fixed, so this page has exactly one verb. Clearing the field is
// meaningful too: it restores the colour's own name.
async function rename(c: Category, value: string) {
  const name = value.trim()
  if (name === c.name) return
  busy.value = true
  try {
    const saved = await api.renameCategory(c.id, name)
    const i = categories.value.findIndex((x) => x.id === c.id)
    if (i >= 0) categories.value[i] = saved
  } catch (e) {
    await reload() // drop the optimistic edit
    toast.error(t('settings.category_save_error'), { description: String(e) })
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <Card class="p-6 space-y-4">
    <h2 class="text-lg font-semibold">{{ t('settings.section.categories') }}</h2>
    <p class="text-sm text-muted-foreground">{{ t('settings.categories_help') }}</p>

    <ul class="text-sm divide-y">
      <li v-for="c in categories" :key="c.id" class="py-2 flex items-center gap-3">
        <CategoryDot :color="c.color" size="md" />
        <input
          :value="c.name"
          :placeholder="labelFor(c)"
          :aria-label="t('settings.category_name')"
          :disabled="busy"
          maxlength="64"
          class="flex-1 min-w-0 rounded-sm border border-transparent bg-transparent px-2 py-1 text-sm
            hover:border-hairline focus:border-hairline focus:outline-none focus:ring-2 focus:ring-ring
            disabled:opacity-50"
          @keyup.enter="($event.target as HTMLInputElement).blur()"
          @blur="rename(c, ($event.target as HTMLInputElement).value)"
        />
      </li>
    </ul>
  </Card>
</template>
