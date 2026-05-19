import { createI18n } from 'vue-i18n'
import en from './locales/en.json'
import ko from './locales/ko.json'

export type Locale = 'en' | 'ko'

const STORAGE_KEY = 'locale'

function detectInitial(): Locale {
  const stored = localStorage.getItem(STORAGE_KEY)
  if (stored === 'en' || stored === 'ko') return stored
  return navigator.language?.toLowerCase().startsWith('ko') ? 'ko' : 'en'
}

const initial = detectInitial()

export const i18n = createI18n({
  legacy: false,
  locale: initial,
  fallbackLocale: 'en',
  messages: { en, ko },
})

document.documentElement.lang = initial

export function setLocale(loc: Locale) {
  i18n.global.locale.value = loc
  localStorage.setItem(STORAGE_KEY, loc)
  document.documentElement.lang = loc
}
