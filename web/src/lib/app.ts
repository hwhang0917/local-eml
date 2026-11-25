export const APP_NAME = 'Local Eml'
export const REPO_URL = 'https://github.com/hwhang0917/local-eml'
export const DOCS_URL = 'https://hwhang0917.github.io/local-eml/docs/'

// The docs site keeps Korean pages under /ko/.
export function docsUrl(locale: string): string {
  return locale === 'ko' ? `${DOCS_URL}ko/` : DOCS_URL
}

export function pageTitle(part?: string): string {
  return part ? `${APP_NAME} | ${part}` : APP_NAME
}
