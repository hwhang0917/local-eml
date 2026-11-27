export const APP_NAME = 'Local Eml'
export const REPO_URL = 'https://github.com/hwhang0917/local-eml'
export const DOCS_URL = 'https://hwhang0917.github.io/local-eml/docs/'

// The docs site keeps translated pages under /<locale>/; English is the root.
const DOCS_LOCALES = ['ko', 'ja']

export function docsUrl(locale: string): string {
  return DOCS_LOCALES.includes(locale) ? `${DOCS_URL}${locale}/` : DOCS_URL
}

export function pageTitle(part?: string): string {
  return part ? `${APP_NAME} | ${part}` : APP_NAME
}
