<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatDate, formatEpoch } from '@/lib/format'
import { cn } from '@/lib/utils'
import type { IMAPProfile } from '@/lib/api'
import Button from '@/components/ui/Button.vue'
import Switch from '@/components/ui/Switch.vue'

const props = defineProps<{
  profile: IMAPProfile
  selected: boolean
  /** A save of this profile's sync flag is in flight. */
  toggling: boolean
  syncing: boolean
}>()
const emit = defineEmits<{
  select: []
  'toggle-sync': [enabled: boolean]
  'sync-now': []
}>()

const { t } = useI18n()

const address = computed(() => {
  const p = props.profile
  const port = p.port && p.port !== 993 ? `:${p.port}` : ''
  return `${p.username} @ ${p.host}${port}`
})

// Short form on the card (follows the date-format setting); the long one is
// in the tooltip.
const lastSynced = computed(() =>
  props.profile.last_synced_at ? formatDate(new Date(props.profile.last_synced_at * 1000).toISOString()) : '',
)

// How new mail reaches this profile, when the server reports it: pushed live
// (IMAP IDLE), polled because the server can't push, or reconnecting. While
// the push connection is still being set up the card says plain "sync on".
const push = computed(() => {
  switch (props.profile.push) {
    case 'live':
      return { label: t('import.imap_card_push_live'), help: t('import.imap_card_push_live_help'), tone: 'live' }
    case 'retrying':
      return { label: t('import.imap_card_push_retrying'), help: t('import.imap_card_push_retrying_help'), tone: 'warn' }
    case 'unsupported':
      return { label: t('import.imap_card_push_unsupported'), help: t('import.imap_card_push_unsupported_help'), tone: 'on' }
    default:
      return { label: t('import.imap_card_sync_on'), help: undefined, tone: 'on' }
  }
})

// Sync is switched on but the poller can't log in yet: the password is only
// kept after the first import with it.
const waitingForPassword = computed(() => props.profile.sync_enabled && !props.profile.has_password)
</script>

<!--
  One saved IMAP profile. Clicking the card loads it into the import form
  below; the switch turns background sync on or off for it straight away.
-->
<template>
  <div
    :class="cn(
      'rounded-md border bg-card p-3 flex flex-col gap-2 transition-colors',
      selected ? 'border-primary ring-1 ring-primary' : 'border-border hover:border-muted-foreground/50',
    )"
    data-testid="imap-profile-card"
  >
    <div class="flex items-start gap-3">
      <button
        type="button"
        class="flex-1 min-w-0 text-left cursor-pointer focus-visible:outline-none focus-visible:underline"
        :aria-pressed="selected"
        :title="t('import.imap_profile_select')"
        @click="emit('select')"
      >
        <span class="flex items-center gap-2">
          <span
            :class="cn(
              'inline-block h-2 w-2 rounded-full shrink-0',
              profile.sync_enabled ? (waitingForPassword ? 'bg-amber-500' : 'bg-emerald-500') : 'bg-muted-foreground/30',
            )"
            aria-hidden="true"
          />
          <span class="font-medium truncate">{{ profile.name }}</span>
        </span>
        <span class="block text-xs text-muted-foreground font-mono truncate mt-0.5" :title="address">
          {{ address }} · {{ profile.folder || 'INBOX' }}
        </span>
      </button>
      <Switch
        :model-value="profile.sync_enabled"
        :disabled="toggling"
        :aria-label="t('import.imap_card_sync_toggle', { name: profile.name })"
        @update:model-value="(v: boolean) => emit('toggle-sync', v)"
      />
    </div>

    <div class="flex items-center gap-2 text-xs min-h-8">
      <template v-if="!profile.sync_enabled">
        <span class="text-muted-foreground">{{ t('import.imap_card_sync_off') }}</span>
      </template>
      <template v-else-if="waitingForPassword">
        <span class="text-amber-600">{{ t('import.imap_sync_needs_password') }}</span>
      </template>
      <template v-else>
        <span
          class="flex-1 min-w-0 truncate text-muted-foreground"
          :title="profile.last_synced_at ? formatEpoch(profile.last_synced_at) : undefined"
        >
          <span
            :class="cn(
              'font-medium',
              push.tone === 'warn' ? 'text-amber-600' : 'text-emerald-700 dark:text-emerald-400',
            )"
            :title="push.help"
            data-testid="imap-push-state"
          >
            {{ push.label }}
            <span v-if="push.help" class="sr-only">: {{ push.help }}</span>
          </span>
          ·
          {{ profile.last_synced_at
            ? t('import.imap_card_last_synced', { when: lastSynced })
            : t('import.imap_card_never_synced') }}
        </span>
        <Button
          size="sm"
          variant="outline"
          class="shrink-0"
          :disabled="syncing"
          @click="emit('sync-now')"
        >
          {{ syncing ? t('import.imap_sync_running') : t('import.imap_sync_now') }}
        </Button>
      </template>
    </div>
  </div>
</template>
