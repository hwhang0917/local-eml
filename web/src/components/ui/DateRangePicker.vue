<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CalendarDays, ChevronLeft, ChevronRight, X } from 'lucide-vue-next'
import { CalendarDate } from '@internationalized/date'
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
  RangeCalendarHeading,
  RangeCalendarNext,
  RangeCalendarPrev,
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

function clear() {
  range.value = { start: undefined, end: undefined }
  emit('change', { from: '', to: '' })
  open.value = false
}
</script>

<template>
  <PopoverRoot v-model:open="open">
    <PopoverTrigger
      data-tour="dates"
      :aria-label="t('library.date_range')"
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
        class="z-50 rounded-lg border border-hairline bg-background p-4 shadow-lg"
      >
        <PopoverArrow class="fill-background" />
        <RangeCalendarRoot
          v-slot="{ grid, weekDays }"
          v-model="range"
          :locale="locale"
          :number-of-months="2"
          fixed-weeks
          class="select-none"
        >
          <RangeCalendarHeader class="relative mb-3 flex items-center justify-center">
            <RangeCalendarPrev
              :aria-label="t('library.prev')"
              class="absolute left-0 inline-flex h-7 w-7 items-center justify-center rounded-sm hover:bg-accent"
            >
              <ChevronLeft class="h-4 w-4" />
            </RangeCalendarPrev>
            <RangeCalendarHeading class="text-sm font-semibold" />
            <RangeCalendarNext
              :aria-label="t('library.next')"
              class="absolute right-0 inline-flex h-7 w-7 items-center justify-center rounded-sm hover:bg-accent"
            >
              <ChevronRight class="h-4 w-4" />
            </RangeCalendarNext>
          </RangeCalendarHeader>

          <div class="flex flex-col gap-4 sm:flex-row">
            <RangeCalendarGrid v-for="month in grid" :key="month.value.toString()" class="w-full border-collapse">
              <RangeCalendarGridHead>
                <RangeCalendarGridRow class="mb-1 flex w-full justify-between">
                  <RangeCalendarHeadCell
                    v-for="day in weekDays"
                    :key="day"
                    class="w-8 text-center text-xs font-normal text-muted-foreground"
                  >
                    {{ day }}
                  </RangeCalendarHeadCell>
                </RangeCalendarGridRow>
              </RangeCalendarGridHead>
              <RangeCalendarGridBody>
                <RangeCalendarGridRow
                  v-for="(weekDates, index) in month.rows"
                  :key="`weekDate-${index}`"
                  class="flex w-full"
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
                      class="relative inline-flex h-8 w-8 items-center justify-center rounded-sm
                        hover:bg-accent
                        focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring
                        data-[outside-view]:text-muted-foreground/40
                        data-[unavailable]:pointer-events-none data-[unavailable]:line-through
                        data-[selected]:not-data-[selection-start]:not-data-[selection-end]:bg-primary/10
                        data-[highlighted]:not-data-[selection-start]:not-data-[selection-end]:bg-primary/10
                        data-[selection-end]:bg-primary data-[selection-end]:text-primary-foreground
                        data-[selection-start]:bg-primary data-[selection-start]:text-primary-foreground
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
