<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { ShieldAlert } from 'lucide-vue-next'
import { api, type Email } from '@/lib/api'
import { formatDate, riskWarnings, senderName } from '@/lib/format'
import Card from '@/components/ui/Card.vue'
import Button from '@/components/ui/Button.vue'

const { t } = useI18n()
const items = ref<Email[]>([])
const loading = ref(false)
const busy = ref<string | null>(null)

// ponytail: one page of 200, newest first. Paginate if someone flags more.
async function load() {
  loading.value = true
  try {
    const res = await api.listEmails({ flagged: true, sort: 'sent_at', order: 'desc', limit: 200 })
    items.value = res.items
  } catch (e) {
    toast.error(t('settings.flagged_load_error'), { description: String(e) })
  } finally {
    loading.value = false
  }
}

async function unflag(e: Email) {
  busy.value = e.sha256
  try {
    await api.setFlag(e.sha256, '')
    items.value = items.value.filter((x) => x.sha256 !== e.sha256)
    toast.success(t('settings.unflagged'))
  } catch (err) {
    toast.error(t('viewer.flag_error'), { description: String(err) })
  } finally {
    busy.value = null
  }
}

onMounted(load)
</script>

<template>
  <Card class="p-6 space-y-4">
    <h2 class="text-lg font-semibold">{{ t('settings.section.flagged') }}</h2>
    <p class="text-sm text-muted-foreground">{{ t('settings.flagged_help') }}</p>

    <label class="flex items-start gap-2 text-sm">
      <input v-model="riskWarnings" type="checkbox" class="mt-1" />
      <span>
        {{ t('settings.risk_warnings') }}
        <span class="block text-xs text-muted-foreground">{{ t('settings.risk_warnings_help') }}</span>
      </span>
    </label>

    <hr class="border-border" />

    <p v-if="!loading && items.length === 0" class="text-sm text-muted-foreground">
      {{ t('settings.flagged_empty') }}
    </p>

    <ul v-else class="divide-y divide-hairline">
      <li v-for="e in items" :key="e.sha256" class="flex items-center gap-3 py-2.5 text-sm">
        <span
          :class="['inline-flex shrink-0 items-center gap-1 rounded-sm px-1.5 py-0.5 text-xs font-medium',
            e.flag === 'phishing' ? 'bg-destructive/10 text-destructive' : 'bg-amber-500/15 text-amber-700 dark:text-amber-400']"
        >
          <ShieldAlert class="h-3 w-3" />
          {{ t(`viewer.flag_${e.flag}`) }}
        </span>
        <span class="w-44 shrink-0 text-muted-foreground whitespace-nowrap tabular-nums">{{ formatDate(e.sent_at) }}</span>
        <span class="w-36 shrink-0 truncate" :title="e.from">{{ senderName(e.from) }}</span>
        <RouterLink
          :to="{ name: 'viewer', params: { sha: e.sha256 } }"
          class="min-w-0 flex-1 truncate hover:underline"
        >{{ e.subject || t('library.no_subject') }}</RouterLink>
        <Button size="sm" variant="outline" :disabled="busy === e.sha256" @click="unflag(e)">
          {{ t('settings.unflag') }}
        </Button>
      </li>
    </ul>
  </Card>
</template>
