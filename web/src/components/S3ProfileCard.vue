<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { KeyRound, Server } from 'lucide-vue-next'
import type { S3Profile } from '@/lib/api'
import ProfileCard from '@/components/ProfileCard.vue'

const props = defineProps<{
  profile: S3Profile
  selected: boolean
}>()
const emit = defineEmits<{ select: [] }>()

const { t } = useI18n()

const location = computed(() => {
  const prefix = (props.profile.prefix ?? '').replace(/^\/+/, '')
  return `s3://${props.profile.bucket}/${prefix}`
})

// Only the key id is saved, never the secret, so a profile with a key asks
// for the secret each time; one without uses the machine's AWS credentials.
const maskedKey = computed(() => {
  const k = props.profile.access_key_id?.trim() ?? ''
  return k.length > 8 ? `${k.slice(0, 4)}…${k.slice(-4)}` : k
})
</script>

<!-- One saved S3 profile. Clicking the card loads it into the form below. -->
<template>
  <ProfileCard
    :name="profile.name"
    :detail="location"
    :selected="selected"
    :select-title="t('import.s3_profile_select')"
    data-testid="s3-profile-card"
    @select="emit('select')"
  >
    <template #action>
      <span
        class="shrink-0 rounded-sm bg-muted px-1.5 py-0.5 text-[11px] font-mono text-muted-foreground"
        :title="t('import.s3_region')"
      >
        {{ profile.region || t('import.s3_card_region_default') }}
      </span>
    </template>

    <template #footer>
      <span
        v-if="maskedKey"
        class="flex items-center gap-1.5 min-w-0 text-muted-foreground"
        :title="t('import.s3_card_key_help')"
      >
        <KeyRound class="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
        <span class="truncate">{{ t('import.s3_card_key', { key: maskedKey }) }}</span>
      </span>
      <span
        v-else
        class="flex items-center gap-1.5 min-w-0 text-muted-foreground"
        :title="t('import.s3_card_system_help')"
      >
        <Server class="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
        <span class="truncate">{{ t('import.s3_card_system') }}</span>
      </span>
    </template>
  </ProfileCard>
</template>
