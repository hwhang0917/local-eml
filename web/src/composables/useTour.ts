import { useStorage } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { driver } from 'driver.js'
import 'driver.js/dist/driver.css'

const seen = useStorage('tour-seen', false)

export function useTour() {
  const { t } = useI18n()

  function start() {
    seen.value = true
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
    if (!seen.value) start()
  }

  return { start, startIfFirstVisit }
}
