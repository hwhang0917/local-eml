import { useStorage } from '@vueuse/core'
import { useI18n } from 'vue-i18n'

const seen = useStorage('tour-seen', false)

export function useTour() {
  const { t } = useI18n()

  // driver.js is ~50 KiB that most sessions never run, so it loads on demand
  // the first time a tour actually starts.
  async function start() {
    seen.value = true
    const [{ driver }] = await Promise.all([
      import('driver.js'),
      import('driver.js/dist/driver.css'),
    ])
    driver({
      showProgress: true,
      nextBtnText: t('tour.next'),
      prevBtnText: t('tour.prev'),
      doneBtnText: t('tour.done'),
      // Nav first, left to right, then the library controls in the order they
      // sit in the filter bar, then how to get the tour back.
      steps: ['library', 'import', 'export', 'settings', 'search', 'starred', 'dates', 'categories', 'help'].map(
        (key) => ({
          element: `[data-tour="${key}"]`,
          popover: { title: t(`tour.${key}.title`), description: t(`tour.${key}.body`) },
        }),
      ),
    }).drive()
  }

  function startIfFirstVisit() {
    if (!seen.value) void start()
  }

  return { start, startIfFirstVisit }
}
