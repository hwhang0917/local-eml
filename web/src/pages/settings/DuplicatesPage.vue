<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { Copy, Star } from 'lucide-vue-next'
import { api, type DuplicateGroup, type DuplicateSummary } from '@/lib/api'
import { formatBytes, formatDate, formatDateAbsolute, senderName } from '@/lib/format'
import { useSession } from '@/composables/useSession'
import Card from '@/components/ui/Card.vue'
import Button from '@/components/ui/Button.vue'
import ConfirmDialog from '@/components/ui/ConfirmDialog.vue'

// The same email stored more than once, usually because it came in by two
// roads (a Takeout .mbox and IMAP sync). Each group keeps one copy; stars,
// read state and the category carry over to it before the others go.

const { t } = useI18n()
const { canDeleteMail } = useSession()
const PAGE = 50

const summary = ref<DuplicateSummary | null>(null)
const groups = ref<DuplicateGroup[]>([])
// Per group, the copy to keep, starting at the server's pick.
const keep = ref(new Map<string, string>())
const loading = ref(false)
const busy = ref(false)
const confirmAll = ref(false)
const offset = ref(0)

async function load() {
  loading.value = true
  try {
    const r = await api.listDuplicates(PAGE, offset.value)
    summary.value = r.summary
    groups.value = r.groups
    keep.value = new Map(r.groups.map((g) => [g.key, g.keep]))
  } catch (e) {
    toast.error(t('duplicates.load_error'), { description: String(e) })
  } finally {
    loading.value = false
  }
}
onMounted(load)

async function merge(body: Parameters<typeof api.mergeDuplicates>[0]) {
  busy.value = true
  try {
    const r = await api.mergeDuplicates(body)
    toast.success(t('duplicates.merged', { count: r.removed }, r.removed))
    if (offset.value > 0 && groups.value.length <= 1) offset.value = Math.max(0, offset.value - PAGE)
    await load()
  } catch (e) {
    toast.error(t('duplicates.merge_error'), { description: String(e) })
  } finally {
    busy.value = false
  }
}

function mergeOne(g: DuplicateGroup) {
  void merge({ groups: [{ group: g.key, keep: keep.value.get(g.key) ?? g.keep }] })
}

function mergePage() {
  void merge({ groups: groups.value.map((g) => ({ group: g.key, keep: keep.value.get(g.key) ?? g.keep })) })
}

function mergeAll() {
  confirmAll.value = false
  void merge({ all: true })
}

const canPrev = computed(() => offset.value > 0)
const canNext = computed(() => !!summary.value && offset.value + PAGE < summary.value.groups)
function page(dir: 1 | -1) {
  offset.value = Math.max(0, offset.value + dir * PAGE)
  void load()
}
</script>

<template>
  <Card class="p-6 space-y-4">
    <h2 class="text-lg font-semibold">{{ t('settings.section.duplicates') }}</h2>
    <p class="text-sm text-muted-foreground">{{ t('duplicates.help') }}</p>

    <div
      v-if="summary && summary.groups > 0"
      class="flex flex-wrap items-center gap-3 rounded-lg border border-hairline bg-muted/40 p-3"
      data-testid="duplicates-summary"
    >
      <Copy class="h-5 w-5 shrink-0 text-muted-foreground" />
      <p class="min-w-0 flex-1 text-sm">
        {{ t('duplicates.summary', { groups: summary.groups, extra: summary.extra, size: formatBytes(summary.extra_bytes) }, summary.extra) }}
      </p>
      <Button
        size="sm"
        :disabled="busy || !canDeleteMail"
        :title="canDeleteMail ? undefined : t('duplicates.temporary')"
        data-testid="merge-all"
        @click="confirmAll = true"
      >
        {{ t('duplicates.merge_all', { count: summary.extra }, summary.extra) }}
      </Button>
    </div>

    <p v-if="!loading && summary && summary.groups === 0" class="text-sm text-muted-foreground" data-testid="duplicates-empty">
      {{ t('duplicates.empty') }}
    </p>

    <ul v-else class="space-y-3">
      <li v-for="g in groups" :key="g.key" class="rounded-lg border border-hairline p-3" data-testid="duplicate-group">
        <div class="mb-2 flex items-baseline gap-3">
          <RouterLink
            :to="{ name: 'viewer', params: { sha: keep.get(g.key) ?? g.keep } }"
            class="min-w-0 flex-1 truncate font-medium hover:underline"
          >{{ g.items[0].subject || t('library.no_subject') }}</RouterLink>
          <span class="shrink-0 text-xs text-muted-foreground">
            {{ senderName(g.items[0].from) }} · {{ formatDate(g.items[0].sent_at) }}
          </span>
          <Button size="sm" variant="outline" :disabled="busy || !canDeleteMail" @click="mergeOne(g)">
            {{ t('duplicates.merge_one', { count: g.items.length - 1 }, g.items.length - 1) }}
          </Button>
        </div>
        <fieldset class="grid gap-1">
          <legend class="sr-only">{{ t('duplicates.which_to_keep') }}</legend>
          <label
            v-for="e in g.items"
            :key="e.sha256"
            class="flex cursor-pointer items-center gap-3 rounded-sm px-2 py-1 text-sm hover:bg-accent/50"
          >
            <input
              type="radio"
              class="accent-primary"
              :name="g.key"
              :value="e.sha256"
              :checked="(keep.get(g.key) ?? g.keep) === e.sha256"
              @change="keep.set(g.key, e.sha256)"
            />
            <span class="w-16 shrink-0 text-xs font-medium">
              {{ (keep.get(g.key) ?? g.keep) === e.sha256 ? t('duplicates.keep') : t('duplicates.remove') }}
            </span>
            <span class="min-w-0 flex-1 truncate font-mono text-xs text-muted-foreground" :title="e.filename">{{ e.filename || e.sha256.slice(0, 12) }}</span>
            <Star v-if="e.starred" class="h-3.5 w-3.5 shrink-0 text-amber-500" fill="currentColor" />
            <span class="w-20 shrink-0 text-right text-xs tabular-nums text-muted-foreground">{{ formatBytes(e.size_bytes) }}</span>
            <span class="w-56 shrink-0 whitespace-nowrap text-right text-xs text-muted-foreground" :title="formatDateAbsolute(e.imported_at)">
              {{ t('duplicates.imported', { date: formatDate(e.imported_at) }) }}
            </span>
          </label>
        </fieldset>
      </li>
    </ul>

    <div v-if="summary && summary.groups > PAGE || groups.length > 1" class="flex items-center justify-between gap-2">
      <Button v-if="groups.length > 1" size="sm" variant="outline" :disabled="busy || !canDeleteMail" @click="mergePage">
        {{ t('duplicates.merge_page') }}
      </Button>
      <div v-if="summary && summary.groups > PAGE" class="ml-auto flex gap-2">
        <Button size="sm" variant="outline" :disabled="!canPrev || loading" @click="page(-1)">{{ t('library.prev') }}</Button>
        <Button size="sm" variant="outline" :disabled="!canNext || loading" @click="page(1)">{{ t('library.next') }}</Button>
      </div>
    </div>

    <ConfirmDialog
      v-model:open="confirmAll"
      :title="t('duplicates.confirm_title', { count: summary?.extra ?? 0 }, summary?.extra ?? 0)"
      :description="t('duplicates.confirm_body')"
      :confirm-label="t('duplicates.confirm_ok')"
      :cancel-label="t('duplicates.confirm_cancel')"
      destructive
      @confirm="mergeAll"
    />
  </Card>
</template>
