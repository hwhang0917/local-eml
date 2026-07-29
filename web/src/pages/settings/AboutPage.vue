<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { Github, RefreshCw, Download, CheckCircle2, AlertCircle, FolderSync } from 'lucide-vue-next'
import { APP_VERSION } from '@/version'
import { api, type UpdateStatus } from '@/lib/api'
import Card from '@/components/ui/Card.vue'
import Button from '@/components/ui/Button.vue'

const { t } = useI18n()
const origin = window.location.origin
const REPO_URL = 'https://github.com/hwhang0917/local-eml'

const update = ref<UpdateStatus | null>(null)
const checking = ref(false)
const installing = ref(false)

async function refreshUpdate(force = false) {
  checking.value = true
  try {
    update.value = await api.checkUpdate(force)
  } catch (e) {
    toast.error(t('settings.update_check_error'), { description: String(e) })
  } finally {
    checking.value = false
  }
}

async function installUpdate() {
  if (!update.value?.has_update || !update.value.can_install) return
  if (!window.confirm(t('settings.update_confirm', { from: update.value.current, to: update.value.latest }))) return
  installing.value = true
  try {
    await api.installUpdate()
    toast.success(t('settings.update_restarting'))
    // Server is exiting; the offline banner picks up the gap, then useHealth
    // notices when the service manager respawns us on the new binary.
  } catch (e) {
    toast.error(t('settings.update_install_error'), { description: String(e) })
    installing.value = false
  }
}

onMounted(() => refreshUpdate(false))

const resyncing = ref(false)

// Starts the rescan job, then polls it to completion so the toast can carry
// the processed/duplicate/error summary the import pages already show.
async function resyncLibrary() {
  resyncing.value = true
  try {
    const { import_id } = await api.resync()
    for (;;) {
      await new Promise((r) => setTimeout(r, 1000))
      const st = await api.importStatus(import_id)
      if (st.status !== 'done' && st.status !== 'error') continue
      const summary = t('import.toast_summary', {
        processed: st.processed, dup: st.duplicates, err: st.errors,
      })
      if (st.status === 'done' && st.errors === 0) {
        toast.success(t('settings.resync_done'), { description: summary })
      } else {
        toast.error(t('settings.resync_error'), { description: summary })
      }
      return
    }
  } catch (e) {
    toast.error(t('settings.resync_error'), { description: String(e) })
  } finally {
    resyncing.value = false
  }
}

const latestLabel = computed(() => {
  const u = update.value
  if (!u) return ''
  if (u.error) return t('settings.update_check_unavailable')
  if (!u.latest) return t('settings.update_check_unavailable')
  return u.latest
})
</script>

<template>
  <Card class="p-6 space-y-4">
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

    <hr class="border-border" />

    <div class="space-y-2">
      <div class="flex items-center gap-3">
        <h3 class="text-sm font-medium">{{ t('settings.resync_section') }}</h3>
        <Button size="sm" variant="outline" :disabled="resyncing" @click="resyncLibrary">
          <FolderSync class="h-3.5 w-3.5" :class="resyncing ? 'animate-spin' : ''" />
          <span class="ml-1.5">{{ resyncing ? t('settings.resync_running') : t('settings.resync') }}</span>
        </Button>
      </div>
      <p class="text-xs text-muted-foreground">{{ t('settings.resync_help') }}</p>
    </div>

    <hr class="border-border" />

    <div class="space-y-3">
      <div class="flex items-center gap-3">
        <h3 class="text-sm font-medium">{{ t('settings.update_section') }}</h3>
        <Button size="sm" variant="outline" :disabled="checking" @click="refreshUpdate(true)">
          <RefreshCw class="h-3.5 w-3.5" :class="checking ? 'animate-spin' : ''" />
          <span class="ml-1.5">{{ checking ? t('settings.update_checking') : t('settings.update_check') }}</span>
        </Button>
      </div>

      <div v-if="update" class="text-sm space-y-2">
        <div class="flex items-center gap-2">
          <span class="text-muted-foreground">{{ t('settings.update_latest') }}</span>
          <span>{{ latestLabel }}</span>
          <a v-if="update.release_url" :href="update.release_url" target="_blank"
             rel="noopener noreferrer" class="text-xs text-primary hover:underline">
            {{ t('settings.update_release_notes') }}
          </a>
        </div>

        <div v-if="update.has_update" class="flex items-center gap-3 rounded-md bg-accent/50 p-3">
          <Download class="h-4 w-4 text-primary shrink-0" />
          <div class="flex-1 text-sm">
            <p>{{ t('settings.update_available', { latest: update.latest }) }}</p>
            <p v-if="!update.can_install && update.install_note"
               class="text-xs text-muted-foreground mt-1">
              {{ update.install_note }}
            </p>
          </div>
          <Button
            v-if="update.can_install"
            size="sm"
            :disabled="installing"
            @click="installUpdate"
          >
            {{ installing ? t('settings.update_installing') : t('settings.update_install') }}
          </Button>
        </div>

        <div v-else-if="!update.error && update.latest"
             class="flex items-center gap-2 text-sm text-muted-foreground">
          <CheckCircle2 class="h-4 w-4 text-emerald-500" />
          <span>{{ t('settings.update_uptodate') }}</span>
        </div>

        <div v-else-if="update.error"
             class="flex items-center gap-2 text-sm text-muted-foreground">
          <AlertCircle class="h-4 w-4 text-amber-500" />
          <span>{{ t('settings.update_check_failed', { err: update.error }) }}</span>
        </div>
      </div>
    </div>
  </Card>
</template>
