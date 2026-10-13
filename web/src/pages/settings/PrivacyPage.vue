<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { api } from '@/lib/api'
import { useSession } from '@/composables/useSession'
import Card from '@/components/ui/Card.vue'
import Button from '@/components/ui/Button.vue'

const { t } = useI18n()
const { session, refresh } = useSession()

const isService = computed(() => session.value?.mode === 'service')
const isTemporary = computed(() => session.value?.mode === 'temporary')
const busy = ref(false)

const enableDialog = ref<HTMLDialogElement | null>(null)
const alsoWipe = ref(false)
const wipeDialog = ref<HTMLDialogElement | null>(null)
const typed = ref('')
// Typing the word, not just clicking, because this is the one button in the
// app that can't be undone.
const wipeConfirmed = computed(() => typed.value.trim().toLowerCase() === t('privacy.wipe_word'))

// The checkbox only reflects the saved state; it changes after the server
// agrees, so a refused or cancelled change never shows as applied.
function onToggle() {
  if (!session.value || isService.value || busy.value) return
  if (session.value.temporary_launches) {
    void setTemporary(false, false)
  } else {
    alsoWipe.value = false
    enableDialog.value?.showModal()
  }
}

async function setTemporary(enabled: boolean, wipe: boolean) {
  busy.value = true
  try {
    await api.setTemporaryLaunches(enabled)
    await refresh()
    toast.success(t(enabled ? 'privacy.temp_on' : 'privacy.temp_off'))
    enableDialog.value?.close()
    if (wipe) await wipe_()
  } catch (e) {
    toast.error(t('privacy.temp_error'), { description: String(e) })
  } finally {
    busy.value = false
  }
}

function openWipe() {
  typed.value = ''
  wipeDialog.value?.showModal()
}

async function wipe_() {
  busy.value = true
  try {
    const { emails } = await api.wipeData()
    toast.success(t('privacy.wipe_done', { count: emails }))
    wipeDialog.value?.close()
  } catch (e) {
    toast.error(t('privacy.wipe_error'), { description: String(e) })
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="space-y-6">
    <Card class="p-6 space-y-4">
      <h2 class="text-lg font-semibold">{{ t('privacy.temp_title') }}</h2>
      <p class="text-sm text-muted-foreground">{{ t('privacy.temp_help') }}</p>

      <p v-if="isTemporary" class="rounded-sm bg-amber-500/15 p-3 text-sm text-amber-900 dark:text-amber-200">
        {{ t('privacy.temp_now') }}
      </p>

      <label :class="['flex items-start gap-2 text-sm', isService ? 'opacity-60' : '']">
        <input
          type="checkbox"
          class="mt-1"
          :checked="session?.temporary_launches ?? false"
          :disabled="!session || isService || busy"
          @click.prevent="onToggle"
        />
        <span>
          {{ t('privacy.temp_toggle') }}
          <span class="block text-xs text-muted-foreground">{{ t('privacy.temp_toggle_help') }}</span>
        </span>
      </label>

      <p v-if="isService" class="rounded-sm bg-accent p-3 text-sm">
        {{ t('privacy.temp_service') }}
      </p>
    </Card>

    <Card v-if="!isTemporary" class="p-6 space-y-4">
      <h2 class="text-lg font-semibold">{{ t('privacy.wipe_title') }}</h2>
      <p class="text-sm text-muted-foreground">{{ t('privacy.wipe_help') }}</p>
      <p class="text-sm text-muted-foreground">{{ t('privacy.wipe_kept') }}</p>
      <Button variant="destructive" size="sm" :disabled="busy" @click="openWipe">
        {{ t('privacy.wipe_button') }}
      </Button>
    </Card>

    <p class="text-xs text-muted-foreground">{{ t('privacy.disk_note') }}</p>

    <dialog ref="enableDialog" aria-labelledby="enable-dialog-title"
      class="m-auto w-[90vw] max-w-md rounded-lg border bg-background p-0 text-foreground backdrop:bg-black/50">
      <div class="space-y-3 p-5">
        <h2 id="enable-dialog-title" class="font-semibold">{{ t('privacy.enable_title') }}</h2>
        <p class="text-sm text-muted-foreground">{{ t('privacy.enable_body') }}</p>
        <label class="flex items-start gap-2 text-sm">
          <input v-model="alsoWipe" type="checkbox" class="mt-1" />
          <span>{{ t('privacy.enable_wipe') }}</span>
        </label>
        <div class="flex justify-end gap-2">
          <Button variant="ghost" size="sm" :disabled="busy" @click="enableDialog?.close()">
            {{ t('privacy.cancel') }}
          </Button>
          <Button size="sm" :variant="alsoWipe ? 'destructive' : 'default'" :disabled="busy" @click="setTemporary(true, alsoWipe)">
            {{ t('privacy.enable_confirm') }}
          </Button>
        </div>
      </div>
    </dialog>

    <dialog ref="wipeDialog" aria-labelledby="wipe-dialog-title"
      class="m-auto w-[90vw] max-w-md rounded-lg border bg-background p-0 text-foreground backdrop:bg-black/50">
      <form class="space-y-3 p-5" @submit.prevent="wipeConfirmed && wipe_()">
        <h2 id="wipe-dialog-title" class="font-semibold">{{ t('privacy.wipe_confirm_title') }}</h2>
        <p class="text-sm text-muted-foreground">{{ t('privacy.wipe_confirm_body', { word: t('privacy.wipe_word') }) }}</p>
        <input
          v-model="typed"
          type="text"
          autocomplete="off"
          :aria-label="t('privacy.wipe_confirm_body', { word: t('privacy.wipe_word') })"
          class="w-full rounded-sm border bg-background px-2 py-1 text-sm"
        />
        <div class="flex justify-end gap-2">
          <Button variant="ghost" size="sm" :disabled="busy" @click="wipeDialog?.close()">
            {{ t('privacy.cancel') }}
          </Button>
          <Button type="submit" variant="destructive" size="sm" :disabled="busy || !wipeConfirmed">
            {{ t('privacy.wipe_confirm') }}
          </Button>
        </div>
      </form>
    </dialog>
  </div>
</template>
