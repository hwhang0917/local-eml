<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useStorage } from '@vueuse/core'
import { Github } from 'lucide-vue-next'
import { api, type Tag } from '@/lib/api'
import { setLocale, type Locale } from '@/i18n'
import { APP_VERSION } from '@/version'
import Card from '@/components/ui/Card.vue'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select'
import {
  Sidebar,
  SidebarHeader,
  SidebarTitle,
  SidebarContent,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
} from '@/components/ui/sidebar'

type Section = 'about' | 'language' | 'tags' | 'attributions'

const { t, locale } = useI18n()
const section = useStorage<Section>('settings-section', 'about')

const sections = computed<{ key: Section; label: string }[]>(() => [
  { key: 'about', label: t('settings.section.about') },
  { key: 'language', label: t('settings.section.language') },
  { key: 'tags', label: t('settings.section.tags') },
  { key: 'attributions', label: t('settings.section.attributions') },
])

const origin = window.location.origin
const REPO_URL = 'https://github.com/hwhang0917/local-eml'

const langs = [
  { value: 'en' as Locale, label: 'English' },
  { value: 'ko' as Locale, label: '한국어' },
]
const currentLangLabel = computed(
  () => langs.find((l) => l.value === locale.value)?.label ?? locale.value,
)

const tags = ref<Tag[]>([])
const tagsLoading = ref(true)
onMounted(async () => {
  try { tags.value = await api.listTags() } finally { tagsLoading.value = false }
})

interface Attribution {
  name: string
  url: string
  license: string
}

const attributions: Attribution[] = [
  { name: 'Vue', url: 'https://vuejs.org', license: 'MIT' },
  { name: 'Vite', url: 'https://vitejs.dev', license: 'MIT' },
  { name: 'TypeScript', url: 'https://www.typescriptlang.org', license: 'Apache-2.0' },
  { name: 'Tailwind CSS', url: 'https://tailwindcss.com', license: 'MIT' },
  { name: 'reka-ui', url: 'https://reka-ui.com', license: 'MIT' },
  { name: 'tw-animate-css', url: 'https://github.com/Wombosvideo/tw-animate-css', license: 'MIT' },
  { name: 'Vue Router', url: 'https://router.vuejs.org', license: 'MIT' },
  { name: 'Vue I18n', url: 'https://vue-i18n.intlify.dev', license: 'MIT' },
  { name: '@vueuse/core', url: 'https://vueuse.org', license: 'MIT' },
  { name: 'vue-sonner', url: 'https://github.com/xiaoluoboding/vue-sonner', license: 'MIT' },
  { name: 'lucide', url: 'https://lucide.dev', license: 'ISC' },
  { name: 'Roboto', url: 'https://fonts.google.com/specimen/Roboto', license: 'Apache-2.0' },
  { name: 'Pretendard', url: 'https://github.com/orioncactus/pretendard', license: 'SIL OFL 1.1' },
  { name: 'Go', url: 'https://go.dev', license: 'BSD-3-Clause' },
  { name: 'chi', url: 'https://github.com/go-chi/chi', license: 'MIT' },
  { name: 'enmime', url: 'https://github.com/jhillyerd/enmime', license: 'MIT' },
  { name: 'modernc.org/sqlite', url: 'https://gitlab.com/cznic/sqlite', license: 'BSD-3-Clause' },
  { name: 'bluemonday', url: 'https://github.com/microcosm-cc/bluemonday', license: 'BSD-3-Clause' },
]
</script>

<template>
  <div class="flex gap-6">
    <aside class="w-56 shrink-0 self-start">
      <Sidebar>
        <SidebarHeader>
          <SidebarTitle>{{ t('nav.settings') }}</SidebarTitle>
        </SidebarHeader>
        <SidebarContent>
          <SidebarMenu>
            <SidebarMenuItem v-for="s in sections" :key="s.key">
              <SidebarMenuButton :active="section === s.key" @click="section = s.key">
                {{ s.label }}
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarContent>
      </Sidebar>
    </aside>

    <section class="flex-1 min-w-0">
      <Card v-if="section === 'about'" class="p-6 space-y-4">
        <h2 class="text-lg font-semibold">{{ t('settings.section.about') }}</h2>
        <dl class="text-sm grid grid-cols-[10rem_1fr] gap-y-2">
          <dt class="text-muted-foreground">{{ t('settings.version') }}</dt>
          <dd>v{{ APP_VERSION }}</dd>
          <dt class="text-muted-foreground">{{ t('settings.server') }}</dt>
          <dd><code>{{ origin }}</code></dd>
        </dl>
        <p class="text-xs text-muted-foreground">
          {{ t('settings.data_location', { path: '~/.local-eml/' }) }}
        </p>
        <div>
          <a
            :href="REPO_URL"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-2 text-sm text-primary hover:underline"
          >
            <Github class="h-4 w-4" />
            {{ t('settings.github') }}
          </a>
        </div>
      </Card>

      <Card v-else-if="section === 'language'" class="p-6">
        <h2 class="text-lg font-semibold mb-1">{{ t('settings.section.language') }}</h2>
        <p class="text-sm text-muted-foreground mb-4">{{ t('settings.language_help') }}</p>
        <Select
          :model-value="locale"
          @update:model-value="(v) => v && setLocale(v as Locale)"
        >
          <SelectTrigger class="w-56">
            <SelectValue>{{ currentLangLabel }}</SelectValue>
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="l in langs" :key="l.value" :value="l.value">
              {{ l.label }}
            </SelectItem>
          </SelectContent>
        </Select>
      </Card>

      <Card v-else-if="section === 'tags'" class="p-6">
        <h2 class="text-lg font-semibold mb-4">{{ t('settings.section.tags') }}</h2>
        <p v-if="tagsLoading" class="text-sm text-muted-foreground">{{ t('settings.loading') }}</p>
        <p v-else-if="tags.length === 0" class="text-sm text-muted-foreground">{{ t('settings.no_tags') }}</p>
        <ul v-else class="text-sm divide-y">
          <li v-for="tg in tags" :key="tg.name" class="py-1.5 flex justify-between">
            <span>{{ tg.name }}</span>
            <span class="text-muted-foreground">{{ tg.count }}</span>
          </li>
        </ul>
      </Card>

      <Card v-else-if="section === 'attributions'" class="p-6 space-y-4">
        <h2 class="text-lg font-semibold">{{ t('settings.section.attributions') }}</h2>
        <p class="text-sm text-muted-foreground">{{ t('settings.attributions_intro') }}</p>
        <ul class="text-sm divide-y">
          <li
            v-for="a in attributions"
            :key="a.name"
            class="py-2 flex items-center justify-between gap-3"
          >
            <a
              :href="a.url"
              target="_blank"
              rel="noopener noreferrer"
              class="text-primary hover:underline"
            >{{ a.name }}</a>
            <span class="text-xs text-muted-foreground">{{ a.license }}</span>
          </li>
        </ul>
      </Card>
    </section>
  </div>
</template>
