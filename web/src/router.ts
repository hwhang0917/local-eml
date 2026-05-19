import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  { path: '/', name: 'library', component: () => import('@/pages/LibraryPage.vue') },
  { path: '/email/:sha', name: 'viewer', component: () => import('@/pages/ViewerPage.vue'), props: true },
  { path: '/import', name: 'import', component: () => import('@/pages/ImportPage.vue') },
  { path: '/settings', name: 'settings', component: () => import('@/pages/SettingsPage.vue') },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})
