import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  { path: '/', name: 'library', component: () => import('@/pages/LibraryPage.vue'), meta: { titleKey: 'nav.library' } },
  { path: '/email/:sha', name: 'viewer', component: () => import('@/pages/ViewerPage.vue'), props: true, meta: { titleKey: 'nav.viewer' } },
  { path: '/import', name: 'import', component: () => import('@/pages/ImportPage.vue'), meta: { titleKey: 'nav.import' } },
  { path: '/export', name: 'export', component: () => import('@/pages/ExportPage.vue'), meta: { titleKey: 'nav.export' } },
  { path: '/stats', name: 'stats', component: () => import('@/pages/StatsPage.vue'), meta: { titleKey: 'nav.stats' } },
  {
    path: '/settings',
    component: () => import('@/pages/settings/SettingsLayout.vue'),
    meta: { titleKey: 'nav.settings' },
    children: [
      { path: '', redirect: { name: 'settings-about' } },
      { path: 'about', name: 'settings-about', component: () => import('@/pages/settings/AboutPage.vue'), meta: { titleKey: 'settings.section.about' } },
      { path: 'categories', name: 'settings-categories', component: () => import('@/pages/settings/CategoriesPage.vue'), meta: { titleKey: 'settings.section.categories' } },
      { path: 'locale', name: 'settings-locale', component: () => import('@/pages/settings/LocalePage.vue'), meta: { titleKey: 'settings.section.locale' } },
      { path: 'restore', name: 'settings-restore', component: () => import('@/pages/settings/RestorePage.vue'), meta: { titleKey: 'settings.section.restore' } },
      { path: 'attributions', name: 'settings-attributions', component: () => import('@/pages/settings/AttributionsPage.vue'), meta: { titleKey: 'settings.section.attributions' } },
    ],
  },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})
