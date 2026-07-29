<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CalendarDays, ChevronDown, ChevronLeft, ChevronRight, X } from 'lucide-vue-next'
import { CalendarDate, getLocalTimeZone, today } from '@internationalized/date'
import type { DateRange } from 'reka-ui'
import {
  PopoverArrow,
  PopoverContent,
  PopoverPortal,
  PopoverRoot,
  PopoverTrigger,
  RangeCalendarCell,
  RangeCalendarCellTrigger,
  RangeCalendarGrid,
  RangeCalendarGridBody,
  RangeCalendarGridHead,
  RangeCalendarGridRow,
  RangeCalendarHeadCell,
  RangeCalendarHeader,
  RangeCalendarRoot,
} from 'reka-ui'
import Button from '@/components/ui/Button.vue'

const props = defineProps<{ from: string; to: string }>()
const emit = defineEmits<{ change: [{ from: string; to: string }] }>()

const { t, locale } = useI18n()
const open = ref(false)

// Reading y/m/d only, so a structural type spans both this package's
// CalendarDate and reka-ui's inlined copy of the same declarations.
type DateParts = { year: number; month: number; day: number }

// The API speaks YYYY-MM-DD and the calendar speaks CalendarDate. Neither
// carries a timezone, which is what a "date filter" should mean — parsing
// through Date would shift the boundary by the UTC offset.
//
// reka-ui bundles its own rolled-up declarations of @internationalized/date,
// and the #private brand on those makes them nominally distinct from ours
// even though a single copy of the package is installed. The cast reconciles
// the two declarations of what is, at runtime, the same class.
function toCalendarDate(iso: string): DateRange['start'] {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso)
  if (!m) return undefined
  return new CalendarDate(Number(m[1]), Number(m[2]), Number(m[3])) as unknown as DateRange['start']
}

function toISO(d: DateParts | undefined): string {
  if (!d) return ''
  return `${String(d.year).padStart(4, '0')}-${String(d.month).padStart(2, '0')}-${String(d.day).padStart(2, '0')}`
}

// shallowRef, not ref: deep UnwrapRef rewrites CalendarDate into a plain
// structural type and loses the brand reka-ui's DateValue requires. The range
// is always replaced wholesale, so shallow tracking is also the right depth.
const range = shallowRef<DateRange>({ start: toCalendarDate(props.from), end: toCalendarDate(props.to) })

watch(
  () => [props.from, props.to],
  ([from, to]) => {
    range.value = { start: toCalendarDate(from), end: toCalendarDate(to) }
  },
)

// Airbnb closes once both ends exist. Emitting on every change would refetch
// the library on the first click, when only the start is known.
watch(range, (next) => {
  if (!next?.start || !next?.end) return
  const from = toISO(next.start)
  const to = toISO(next.end)
  if (from === props.from && to === props.to) return
  emit('change', { from, to })
  open.value = false
})

const label = computed(() => {
  if (!props.from && !props.to) return t('library.date_any')
  const fmt = (iso: string) => {
    const d = toCalendarDate(iso)
    if (!d) return '…'
    return new Date(d.year, d.month - 1, d.day).toLocaleDateString(locale.value, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    })
  }
  return `${props.from ? fmt(props.from) : '…'} ~ ${props.to ? fmt(props.to) : '…'}`
})

const hasRange = computed(() => props.from !== '' || props.to !== '')

type RekaDate = NonNullable<DateRange['start']>

// Ray Tomlinson sent the first networked email in 1971. Nothing in a local
// archive can predate that, so it makes a better floor than an arbitrary year
// — and it keeps the year list from scrolling into antiquity.
const FIRST_EMAIL = new CalendarDate(1971, 9, 26) as unknown as RekaDate
const maxDate = today(getLocalTimeZone()) as unknown as RekaDate

const YEARS_PER_PAGE = 12

// Days -> months -> years, the zoom-out ladder behind the heading button.
type View = 'day' | 'month' | 'year'
const view = ref<View>('day')
const placeholder = shallowRef<RekaDate>(range.value.start ?? maxDate)

// Reopening should not strand the user wherever they browsed to last time.
watch(open, (isOpen) => {
  if (!isOpen) return
  view.value = 'day'
  placeholder.value = range.value.start ?? maxDate
})

const monthNames = computed(() => {
  const fmt = new Intl.DateTimeFormat(locale.value, { month: 'short' })
  return Array.from({ length: 12 }, (_, i) => fmt.format(new Date(2001, i, 1)))
})

// Pages are anchored to the floor year so the grid never shifts under you as
// you page back and forth.
const yearPageStart = computed(
  () => placeholder.value.year - ((placeholder.value.year - FIRST_EMAIL.year) % YEARS_PER_PAGE),
)
const yearPage = computed(() =>
  Array.from({ length: YEARS_PER_PAGE }, (_, i) => yearPageStart.value + i),
)

const heading = computed(() => {
  if (view.value === 'year') {
    return `${yearPage.value[0]} – ${yearPage.value[YEARS_PER_PAGE - 1]}`
  }
  return new Intl.DateTimeFormat(locale.value, { year: 'numeric' }).format(
    new Date(placeholder.value.year, 0, 1),
  )
})

function monthLabel(d: DateParts) {
  return new Intl.DateTimeFormat(locale.value, { month: 'long' }).format(
    new Date(d.year, d.month - 1, 1),
  )
}

// Absolute weekday, not the locale's column index: a locale whose week starts
// on Monday would otherwise paint the wrong two columns. These are plain
// classes, so every state variant (selected, disabled, outside-view) carries an
// attribute selector and outranks them.
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

function zoomOut() {
  view.value = view.value === 'day' ? 'month' : 'year'
}

function pickMonth(month: number) {
  // day: 1 avoids overflow — .set({ month: 2 }) from a 31st would otherwise
  // have to be constrained.
  placeholder.value = placeholder.value.set({ month, day: 1 })
  view.value = 'day'
}

function pickYear(year: number) {
  placeholder.value = placeholder.value.set({ year, day: 1 })
  view.value = 'month'
}

function isMonthDisabled(month: number) {
  const first = placeholder.value.set({ month, day: 1 })
  const last = first.add({ months: 1 }).subtract({ days: 1 })
  return last.compare(FIRST_EMAIL) < 0 || first.compare(maxDate) > 0
}

function isYearDisabled(year: number) {
  return year < FIRST_EMAIL.year || year > maxDate.year
}

function page(direction: 1 | -1) {
  if (view.value === 'day') placeholder.value = placeholder.value.add({ months: direction })
  else if (view.value === 'month') placeholder.value = placeholder.value.add({ years: direction })
  else placeholder.value = placeholder.value.add({ years: direction * YEARS_PER_PAGE })
}

// A page is reachable only if some part of it falls inside the allowed span.
const canPagePrev = computed(() => {
  if (view.value === 'year') return yearPageStart.value > FIRST_EMAIL.year
  const prev = view.value === 'day'
    ? placeholder.value.subtract({ months: 1 })
    : placeholder.value.subtract({ years: 1 })
  const end = view.value === 'day' ? prev.add({ months: 1 }) : prev.set({ month: 12, day: 31 })
  return end.compare(FIRST_EMAIL) >= 0
})
const canPageNext = computed(() => {
  if (view.value === 'year') return yearPageStart.value + YEARS_PER_PAGE <= maxDate.year
  const next = view.value === 'day'
    ? placeholder.value.add({ months: 1 })
    : placeholder.value.add({ years: 1 })
  const start = view.value === 'day' ? next : next.set({ month: 1, day: 1 })
  return start.compare(maxDate) <= 0
})

function clear() {
  range.value = { start: undefined, end: undefined }
  emit('change', { from: '', to: '' })
  open.value = false
}
</script>

<template>
  <PopoverRoot v-model:open="open">
    <!-- No aria-label: the visible label text is the accessible name, so the
         two can't drift apart (Lighthouse label-content-name-mismatch). -->
    <PopoverTrigger
      data-tour="dates"
      :class="[
        'inline-flex h-8 items-center gap-2 rounded-sm border border-hairline bg-pearl px-3 text-sm',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
        hasRange ? 'text-foreground' : 'text-muted-foreground',
      ]"
    >
      <CalendarDays class="h-4 w-4" />
      <span>{{ label }}</span>
    </PopoverTrigger>

    <PopoverPortal>
      <PopoverContent
        :side-offset="6"
        align="start"
        class="z-50 rounded-lg border border-hairline bg-background p-4 shadow-lg sm:min-w-[18rem]"
      >
        <PopoverArrow class="fill-background" />
        <RangeCalendarRoot
          v-slot="{ grid, weekDays }"
          v-model="range"
          v-model:placeholder="placeholder"
          :locale="locale"
          :number-of-months="1"
          :min-value="FIRST_EMAIL"
          :max-value="maxDate"
          fixed-weeks
          class="select-none"
        >
          <RangeCalendarHeader class="relative mb-3 flex items-center justify-center">
            <button
              type="button"
              :aria-label="t('library.prev')"
              :disabled="!canPagePrev"
              class="absolute left-0 inline-flex h-7 w-7 items-center justify-center rounded-sm
                hover:bg-accent disabled:pointer-events-none disabled:opacity-30"
              @click="page(-1)"
            >
              <ChevronLeft class="h-4 w-4" />
            </button>
            <button
              type="button"
              :aria-label="t('library.zoom_out')"
              :disabled="view === 'year'"
              class="inline-flex items-center gap-1 rounded-sm px-2 py-1 text-sm font-semibold
                hover:bg-accent disabled:pointer-events-none
                focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              @click="zoomOut"
            >
              <span>{{ heading }}</span>
              <ChevronDown v-if="view !== 'year'" class="h-3.5 w-3.5 opacity-60" />
            </button>
            <button
              type="button"
              :aria-label="t('library.next')"
              :disabled="!canPageNext"
              class="absolute right-0 inline-flex h-7 w-7 items-center justify-center rounded-sm
                hover:bg-accent disabled:pointer-events-none disabled:opacity-30"
              @click="page(1)"
            >
              <ChevronRight class="h-4 w-4" />
            </button>
          </RangeCalendarHeader>

          <div v-if="view === 'month'" class="grid grid-cols-4 gap-1">
            <button
              v-for="(name, i) in monthNames"
              :key="name"
              type="button"
              :disabled="isMonthDisabled(i + 1)"
              :class="[
                'h-10 rounded-sm text-sm disabled:pointer-events-none disabled:opacity-30',
                'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                i + 1 === placeholder.month
                  ? 'bg-primary text-primary-foreground hover:bg-primary/90'
                  : 'hover:bg-accent',
              ]"
              @click="pickMonth(i + 1)"
            >
              {{ name }}
            </button>
          </div>

          <div v-else-if="view === 'year'" class="grid grid-cols-4 gap-1">
            <button
              v-for="year in yearPage"
              :key="year"
              type="button"
              :disabled="isYearDisabled(year)"
              :class="[
                'h-10 rounded-sm text-sm disabled:pointer-events-none disabled:opacity-30',
                'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                year === placeholder.year
                  ? 'bg-primary text-primary-foreground hover:bg-primary/90'
                  : 'hover:bg-accent',
              ]"
              @click="pickYear(year)"
            >
              {{ year }}
            </button>
          </div>

          <div v-else class="flex flex-col gap-4 sm:flex-row">
            <RangeCalendarGrid v-for="month in grid" :key="month.value.toString()" class="w-full border-collapse">
              <caption class="mb-1 text-center text-sm font-medium">{{ monthLabel(month.value) }}</caption>
              <RangeCalendarGridHead>
                <RangeCalendarGridRow class="mb-1 flex w-full justify-between">
                  <RangeCalendarHeadCell
                    v-for="(day, i) in weekDays"
                    :key="day"
                    :class="['w-8 text-center text-xs font-normal',
                      weekendClass(month.rows[0][i]) || 'text-muted-foreground']"
                  >
                    {{ day }}
                  </RangeCalendarHeadCell>
                </RangeCalendarGridRow>
              </RangeCalendarGridHead>
              <RangeCalendarGridBody>
                <!-- Same justify-between as the weekday header row: both rows
                     hold seven 2rem cells, so any other distribution drifts
                     the date columns away from their labels. -->
                <RangeCalendarGridRow
                  v-for="(weekDates, index) in month.rows"
                  :key="`weekDate-${index}`"
                  class="flex w-full justify-between"
                >
                  <RangeCalendarCell
                    v-for="weekDate in weekDates"
                    :key="weekDate.toString()"
                    :date="weekDate"
                    class="relative p-0 text-center text-sm"
                  >
                    <RangeCalendarCellTrigger
                      :day="weekDate"
                      :month="month.value"
                      :class="weekendClass(weekDate)"
                      class="relative inline-flex h-8 w-8 items-center justify-center rounded-sm
                        hover:bg-accent
                        focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring
                        data-[outside-view]:text-muted-foreground/40
                        data-[unavailable]:pointer-events-none data-[unavailable]:line-through
                        data-[disabled]:pointer-events-none data-[disabled]:text-muted-foreground/30
                        data-[selected]:not-data-[selection-start]:not-data-[selection-end]:bg-primary/10
                        data-[highlighted]:not-data-[selection-start]:not-data-[selection-end]:bg-primary/10
                        data-[selection-end]:bg-primary data-[selection-end]:text-primary-foreground
                        data-[selection-start]:bg-primary data-[selection-start]:text-primary-foreground
                        data-[selection-start]:hover:bg-primary data-[selection-end]:hover:bg-primary
                        data-[today]:font-semibold data-[today]:underline"
                    />
                  </RangeCalendarCell>
                </RangeCalendarGridRow>
              </RangeCalendarGridBody>
            </RangeCalendarGrid>
          </div>

          <div class="mt-3 flex justify-end border-t border-hairline pt-3">
            <Button variant="ghost" size="sm" :disabled="!hasRange" @click="clear">
              <X class="h-4 w-4" />
              <span class="ml-1.5">{{ t('library.date_clear') }}</span>
            </Button>
          </div>
        </RangeCalendarRoot>
      </PopoverContent>
    </PopoverPortal>
  </PopoverRoot>
</template>
