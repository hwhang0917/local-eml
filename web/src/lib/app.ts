export const APP_NAME = 'Local Eml'
export const REPO_URL = 'https://github.com/hwhang0917/local-eml'

export function pageTitle(part?: string): string {
  return part ? `${APP_NAME} | ${part}` : APP_NAME
}
