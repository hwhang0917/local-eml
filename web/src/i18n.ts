import { createI18n } from 'vue-i18n'
import en from './locales/en.json'
import ko from './locales/ko.json'
import ja from './locales/ja.json'

export const LOCALES = ['en', 'ko', 'ja'] as const
export type Locale = (typeof LOCALES)[number]

function isLocale(v: string | null | undefined): v is Locale {
  return (LOCALES as readonly string[]).includes(v ?? '')
}

const STORAGE_KEY = 'locale'

function detectInitial(): Locale {
  const stored = localStorage.getItem(STORAGE_KEY)
  if (isLocale(stored)) return stored
  const browser = navigator.language?.toLowerCase().split('-')[0]
  return isLocale(browser) ? browser : 'en'
}

const initial = detectInitial()

export const i18n = createI18n({
  legacy: false,
  locale: initial,
  fallbackLocale: 'en',
  messages: { en, ko, ja },
})

document.documentElement.lang = initial

export function setLocale(loc: Locale) {
  i18n.global.locale.value = loc
  localStorage.setItem(STORAGE_KEY, loc)
  document.documentElement.lang = loc
}
