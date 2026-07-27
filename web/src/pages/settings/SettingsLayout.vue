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
  { name: 'settings-locale', label: t('settings.section.locale') },
  { name: 'settings-attributions', label: t('settings.section.attributions') },
])
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
            <SidebarMenuItem v-for="s in sections" :key="s.name">
              <RouterLink v-slot="{ isActive, navigate }" :to="{ name: s.name }" custom>
                <SidebarMenuButton :active="isActive" @click="navigate">
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
