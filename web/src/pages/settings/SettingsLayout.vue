<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, RouterView } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  Sidebar,
  SidebarHeader,
  SidebarTitle,
  SidebarContent,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
} from '@/components/ui/sidebar'

const { t } = useI18n()

const sections = computed(() => [
  { name: 'settings-about', label: t('settings.section.about') },
  { name: 'settings-categories', label: t('settings.section.categories') },
  { name: 'settings-appearance', label: t('settings.section.appearance') },
  { name: 'settings-locale', label: t('settings.section.locale') },
  { name: 'settings-reading', label: t('settings.section.reading') },
  { name: 'settings-restore', label: t('settings.section.restore') },
  { name: 'settings-duplicates', label: t('settings.section.duplicates') },
  { name: 'settings-flagged', label: t('settings.section.flagged') },
  { name: 'settings-privacy', label: t('settings.section.privacy') },
  { name: 'settings-attributions', label: t('settings.section.attributions') },
])
</script>

<template>
  <!-- Below md the sections sit above the page as a wrapping row, so the page
       keeps the full width instead of losing a quarter of it to the menu. -->
  <div class="flex flex-col gap-4 md:flex-row md:gap-6">
    <aside class="w-full md:w-56 shrink-0 self-start">
      <Sidebar class="w-full md:w-60">
        <SidebarHeader>
          <SidebarTitle>{{ t('nav.settings') }}</SidebarTitle>
        </SidebarHeader>
        <SidebarContent>
          <SidebarMenu class="flex-row flex-wrap md:flex-col">
            <SidebarMenuItem v-for="s in sections" :key="s.name">
              <RouterLink v-slot="{ isActive, navigate }" :to="{ name: s.name }" custom>
                <SidebarMenuButton :active="isActive" class="w-auto md:w-full whitespace-nowrap" @click="navigate">
                  {{ s.label }}
                </SidebarMenuButton>
              </RouterLink>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarContent>
      </Sidebar>
    </aside>

    <section class="flex-1 min-w-0">
      <RouterView />
    </section>
  </div>
</template>
