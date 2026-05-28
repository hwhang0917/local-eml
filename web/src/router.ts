import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  { path: '/', name: 'library', component: () => import('@/pages/LibraryPage.vue'), meta: { titleKey: 'nav.library' } },
  { path: '/email/:sha', name: 'viewer', component: () => import('@/pages/ViewerPage.vue'), props: true, meta: { titleKey: 'nav.viewer' } },
  { path: '/import', name: 'import', component: () => import('@/pages/ImportPage.vue'), meta: { titleKey: 'nav.import' } },
  { path: '/settings', name: 'settings', component: () => import('@/pages/SettingsPage.vue'), meta: { titleKey: 'nav.settings' } },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})
