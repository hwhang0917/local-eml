import { ref } from 'vue'

// The API needs the session cookie the launcher sets through /auth. When a
// request comes back 401 this browser has none for this server, so the shell
// swaps in the sign-in page until the user pastes the token.
export const locked = ref(false)

const AUTH_PATH = '/auth'

// The token goes in the body of a form POST, not the URL: an address with the
// token in it would stay in the browser's history. The server answers with a
// redirect to the app, or a plain "invalid token" page.
export function unlock(token: string) {
  const form = document.createElement('form')
  form.method = 'post'
  form.action = AUTH_PATH
  const field = document.createElement('input')
  field.type = 'hidden'
  field.name = 't'
  field.value = token.trim()
  form.append(field)
  document.body.append(form)
  form.submit()
}
