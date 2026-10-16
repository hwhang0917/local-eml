import { ref } from 'vue'

// The API needs the session cookie the launcher sets through /auth. When a
// request comes back 401 this browser has none for this server, so the shell
// swaps in the sign-in page until the user pastes the token.
export const locked = ref(false)

const AUTH_PATH = '/auth'

export function unlock(token: string) {
  window.location.assign(`${AUTH_PATH}?t=${encodeURIComponent(token.trim())}`)
}
