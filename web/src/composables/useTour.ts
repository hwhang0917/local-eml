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
      steps: [
        {
          element: '[data-tour="import"]',
          popover: { title: t('tour.import.title'), description: t('tour.import.body') },
        },
        {
          element: '[data-tour="search"]',
          popover: { title: t('tour.search.title'), description: t('tour.search.body') },
        },
        {
          element: '[data-tour="starred"]',
          popover: { title: t('tour.starred.title'), description: t('tour.starred.body') },
        },
        {
          element: '[data-tour="export"]',
          popover: { title: t('tour.export.title'), description: t('tour.export.body') },
        },
        {
          element: '[data-tour="settings"]',
          popover: { title: t('tour.settings.title'), description: t('tour.settings.body') },
        },
      ],
    }).drive()
  }

  function startIfFirstVisit() {
    if (!seen.value) start()
  }

  return { start, startIfFirstVisit }
}
