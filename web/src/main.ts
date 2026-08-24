import { createApp } from 'vue'
import App from './App.vue'
import { router } from './router'
import { i18n } from './i18n'
import { APP_NAME, REPO_URL } from './lib/app'
import { APP_VERSION } from './version'
import './style.css'

// Easter egg for whoever opens F12.
console.log(
  `%c📮 ${APP_NAME}%c v${APP_VERSION}%c\nYour mail, your disk, your business.\n%c⭐ ${REPO_URL}`,
  'font-size:20px;font-weight:700;padding:6px 10px;border-radius:6px;background:linear-gradient(135deg,#1e293b,#0f172a);color:#f8fafc;',
  'font-size:12px;margin-left:8px;color:#94a3b8;',
  'font-size:12px;color:#64748b;line-height:2;',
  'font-size:12px;color:#3b82f6;',
)

createApp(App).use(router).use(i18n).mount('#app')
