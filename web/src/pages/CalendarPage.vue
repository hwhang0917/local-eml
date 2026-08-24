<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'
import { getLocalTimeZone, today } from '@internationalized/date'
import {
  CalendarCell,
  CalendarCellTrigger,
  CalendarGrid,
  CalendarGridBody,
  CalendarGridHead,
  CalendarGridRow,
  CalendarHeadCell,
  CalendarHeader,
  CalendarRoot,
} from 'reka-ui'
import type { CalendarRootProps } from 'reka-ui'
import { api, type Email } from '@/lib/api'
import { senderName, weekStartsOn } from '@/lib/format'
import { useListContext } from '@/composables/useListContext'
import Button from '@/components/ui/Button.vue'
import Card from '@/components/ui/Card.vue'

const { t, locale } = useI18n()
const router = useRouter()
const listCtx = useListContext()

// ponytail: DateParts/toISO/weekendClass duplicated from DateRangePicker.vue —
// they exist to bridge reka-ui's rolled-up @internationalized/date types;
// extract to a shared module if a third calendar consumer appears.
type DateParts = { year: number; month: number; day: number }

function toISO(d: DateParts | undefined): string {
  if (!d) return ''
  return `${String(d.year).padStart(4, '0')}-${String(d.month).padStart(2, '0')}-${String(d.day).padStart(2, '0')}`
}

// Absolute weekday, not the locale's column index: a locale whose week starts
// on Monday would otherwise paint the wrong two columns.
function weekendClass(d: DateParts | undefined) {
  if (!d) return ''
  switch (new Date(d.year, d.month - 1, d.day).getDay()) {
    case 0:
      return 'text-red-600 dark:text-red-400'
    case 6:
      return 'text-blue-600 dark:text-blue-400'
    default:
      return ''
  }
}

type RekaDate = NonNullable<CalendarRootProps['placeholder']>

// shallowRef, not ref: deep UnwrapRef strips the brand reka-ui's DateValue
// requires (same reason as DateRangePicker).
const placeholder = shallowRef<RekaDate>(today(getLocalTimeZone()) as RekaDate)
const selected = shallowRef<RekaDate | undefined>()

const counts = ref<Record<string, number>>({})
const error = ref('')

const monthKey = computed(
  () => `${String(placeholder.value.year).padStart(4, '0')}-${String(placeholder.value.month).padStart(2, '0')}`,
)

// Stale counts stay visible while the next month loads — no spinner needed.
watch(
  monthKey,
  async () => {
    try {
      counts.value = await api.getCalendar(monthKey.value)
      error.value = ''
    } catch (e) {
      error.value = String(e)
    }
  },
  { immediate: true },
)

const maxCount = computed(() => Math.max(1, ...Object.values(counts.value)))

function heatClass(iso: string) {
  const c = counts.value[iso]
  if (!c) return ''
  const ratio = c / maxCount.value
  if (ratio <= 1 / 3) return 'bg-primary/10'
  if (ratio <= 2 / 3) return 'bg-primary/20'
  return 'bg-primary/30'
}

// --- selected-day panel ---
const selectedISO = computed(() => toISO(selected.value))
const dayItems = ref<Email[]>([])
const dayTotal = ref(0)
const dayLoading = ref(false)

watch(selectedISO, async (iso) => {
  dayItems.value = []
  dayTotal.value = 0
  if (!iso) return
  dayLoading.value = true
  try {
    const r = await api.listEmails({ from: iso, to: iso, sort: 'sent_at', order: 'asc', limit: 500 })
    dayItems.value = r.items
    dayTotal.value = r.total
    error.value = ''
  } catch (e) {
    error.value = String(e)
  } finally {
    dayLoading.value = false
  }
})

const dayHeading = computed(() => {
  const d = selected.value
  if (!d) return ''
  return new Date(d.year, d.month - 1, d.day).toLocaleDateString(locale.value, {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    weekday: 'short',
  })
})

const timeFmt = computed(() => new Intl.DateTimeFormat(locale.value, { hour: '2-digit', minute: '2-digit' }))

function emailHref(sha: string) {
  return router.resolve({ name: 'viewer', params: { sha } }).href
}

function openEmail(sha: string, index: number, ev: MouseEvent) {
  listCtx.set({
    params: { from: selectedISO.value, to: selectedISO.value, sort: 'sent_at', order: 'asc' },
    index,
    total: dayTotal.value,
  })
  if (ev.ctrlKey || ev.metaKey || ev.shiftKey) {
    window.open(emailHref(sha), '_blank', 'noopener')
    return
  }
  router.push({ name: 'viewer', params: { sha } })
}

function page(direction: 1 | -1) {
  placeholder.value = placeholder.value.add({ months: direction }) as RekaDate
}

function goToday() {
  const now = today(getLocalTimeZone()) as RekaDate
  placeholder.value = now
  selected.value = now
}

const heading = computed(() =>
  new Intl.DateTimeFormat(locale.value, { year: 'numeric', month: 'long' }).format(
    new Date(placeholder.value.year, placeholder.value.month - 1, 1),
  ),
)
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-semibold tracking-tight">{{ t('calendar.title') }}</h1>
    <p v-if="error" class="text-sm text-destructive">{{ error }}</p>

    <div class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_24rem]">
      <Card class="p-6">
        <CalendarRoot
          v-slot="{ grid, weekDays }"
          v-model="selected"
          v-model:placeholder="placeholder"
          :locale="locale"
          :week-starts-on="weekStartsOn"
          fixed-weeks
          class="select-none"
        >
          <CalendarHeader class="relative mb-4 flex items-center justify-center">
            <button
              type="button"
              :aria-label="t('library.prev')"
              class="absolute left-0 inline-flex h-7 w-7 items-center justify-center rounded-sm hover:bg-accent"
              @click="page(-1)"
            >
              <ChevronLeft class="h-4 w-4" />
            </button>
            <span class="text-sm font-semibold">{{ heading }}</span>
            <div class="absolute right-0 flex items-center gap-1">
              <Button variant="ghost" size="sm" @click="goToday">{{ t('calendar.today') }}</Button>
              <button
                type="button"
                :aria-label="t('library.next')"
                class="inline-flex h-7 w-7 items-center justify-center rounded-sm hover:bg-accent"
                @click="page(1)"
              >
                <ChevronRight class="h-4 w-4" />
              </button>
            </div>
          </CalendarHeader>

          <CalendarGrid v-for="month in grid" :key="month.value.toString()" class="w-full border-collapse">
            <CalendarGridHead>
              <CalendarGridRow class="mb-1 grid w-full grid-cols-7">
                <CalendarHeadCell
                  v-for="(day, i) in weekDays"
                  :key="day"
                  :class="['text-center text-xs font-normal',
                    weekendClass(month.rows[0][i]) || 'text-muted-foreground']"
                >
                  {{ day }}
                </CalendarHeadCell>
              </CalendarGridRow>
            </CalendarGridHead>
            <CalendarGridBody class="grid gap-1">
              <CalendarGridRow
                v-for="(weekDates, index) in month.rows"
                :key="`weekDate-${index}`"
                class="grid w-full grid-cols-7 gap-1"
              >
                <CalendarCell
                  v-for="weekDate in weekDates"
                  :key="weekDate.toString()"
                  :date="weekDate"
                  class="relative p-0 text-center text-sm"
                >
                  <CalendarCellTrigger
                    :day="weekDate"
                    :month="month.value"
                    :class="[weekendClass(weekDate), heatClass(toISO(weekDate))]"
                    class="flex h-14 w-full flex-col items-center justify-center gap-0.5 rounded-sm
                      hover:bg-accent
                      focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring
                      data-[outside-view]:text-muted-foreground/40
                      data-[selected]:ring-2 data-[selected]:ring-ring
                      data-[today]:font-semibold data-[today]:underline"
                  >
                    <span>{{ weekDate.day }}</span>
                    <span
                      v-if="counts[toISO(weekDate)]"
                      class="text-[10px] leading-none tabular-nums text-muted-foreground"
                    >
                      {{ counts[toISO(weekDate)] }}
                    </span>
                  </CalendarCellTrigger>
                </CalendarCell>
              </CalendarGridRow>
            </CalendarGridBody>
          </CalendarGrid>
        </CalendarRoot>
      </Card>

      <Card class="p-6">
        <p v-if="!selected" class="text-sm text-muted-foreground">{{ t('calendar.select_day') }}</p>
        <template v-else>
          <div class="mb-3 flex items-baseline justify-between gap-2">
            <h2 class="text-sm font-semibold">{{ dayHeading }}</h2>
            <span class="text-xs tabular-nums text-muted-foreground">
              {{ t('calendar.day_count', { count: dayTotal }) }}
            </span>
          </div>
          <p v-if="dayLoading" class="text-sm text-muted-foreground">{{ t('calendar.loading') }}</p>
          <p v-else-if="dayItems.length === 0" class="text-sm text-muted-foreground">
            {{ t('calendar.empty_day') }}
          </p>
          <ul v-else class="divide-y divide-hairline">
            <li v-for="(e, i) in dayItems" :key="e.sha256">
              <button
                type="button"
                class="flex w-full items-baseline gap-3 rounded-sm px-2 py-2 text-left text-sm hover:bg-accent
                  focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                @click="openEmail(e.sha256, i, $event)"
              >
                <span class="w-12 shrink-0 text-xs tabular-nums text-muted-foreground">
                  {{ timeFmt.format(new Date(e.sent_at)) }}
                </span>
                <span class="min-w-0 flex-1">
                  <span class="block truncate">{{ e.subject || '—' }}</span>
                  <span class="block truncate text-xs text-muted-foreground">{{ senderName(e.from) }}</span>
                </span>
              </button>
            </li>
          </ul>
        </template>
      </Card>
    </div>
  </div>
</template>
