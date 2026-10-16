import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { api, type S3ImportConfig, type S3Profile } from '@/lib/api'

/** Select value for "no saved profile picked" — ids are numeric strings. */
export const NEW_PROFILE = '__new__'

export function emptyS3(): S3ImportConfig {
  return { accessKeyId: '', secretAccessKey: '', sessionToken: '', region: '', bucket: '', prefix: '' }
}

// The bucket form, its saved profiles and the review step, shared by the S3
// import and the S3 export: only the job they start differs.
export function useS3Form() {
  const { t } = useI18n()

  const s3 = ref<S3ImportConfig>(emptyS3())
  const confirming = ref(false)
  const valid = computed(() => s3.value.bucket.trim().length > 0)

  const profiles = ref<S3Profile[]>([])
  const selectedProfileId = ref<number | null>(null)
  const activeProfile = computed(() =>
    profiles.value.find((p) => p.id === selectedProfileId.value) ?? null,
  )
  const deleteConfirmOpen = ref(false)

  async function loadProfiles() {
    try {
      profiles.value = await api.listS3Profiles()
    } catch (e) {
      toast.error(t('import.s3_profile_load_error'), { description: String(e) })
    }
  }
  onMounted(loadProfiles)

  // Secrets are never stored in a profile, so they always start blank.
  function applyProfile(p: S3Profile | null) {
    s3.value = p
      ? { ...emptyS3(), accessKeyId: p.access_key_id ?? '', region: p.region ?? '', bucket: p.bucket, prefix: p.prefix ?? '' }
      : emptyS3()
  }

  function onProfileChange(value: string) {
    if (value === NEW_PROFILE) {
      selectedProfileId.value = null
      applyProfile(null)
      return
    }
    const id = Number(value)
    selectedProfileId.value = id
    applyProfile(profiles.value.find((p) => p.id === id) ?? null)
  }

  const profileSelectValue = computed(() =>
    selectedProfileId.value == null ? NEW_PROFILE : String(selectedProfileId.value),
  )
  const profileSelectLabel = computed(() =>
    activeProfile.value?.name ?? t('import.s3_profile_new'),
  )

  async function saveProfile() {
    const existing = activeProfile.value
    let name = existing?.name ?? ''
    if (!existing) {
      const entered = window.prompt(t('import.s3_profile_save_prompt'), '')
      if (entered == null) return
      name = entered.trim()
      if (!name) return
    }
    try {
      const saved = await api.saveS3Profile({
        name,
        bucket: s3.value.bucket.trim(),
        prefix: s3.value.prefix?.trim() || undefined,
        region: s3.value.region?.trim() || undefined,
        access_key_id: s3.value.accessKeyId?.trim() || undefined,
      })
      await loadProfiles()
      selectedProfileId.value = saved.id
    } catch (e) {
      toast.error(t('import.s3_profile_save_error'), { description: String(e) })
    }
  }

  async function deleteProfile() {
    const p = activeProfile.value
    if (!p) return
    try {
      await api.deleteS3Profile(p.id)
      selectedProfileId.value = null
      applyProfile(null)
      await loadProfiles()
    } catch (e) {
      toast.error(t('import.s3_profile_delete_error'), { description: String(e) })
    }
  }

  const credsLabel = computed(() =>
    s3.value.accessKeyId?.trim() ? t('import.s3_creds_form') : t('import.s3_creds_system'),
  )

  function review() {
    if (valid.value) confirming.value = true
  }

  function cancel() {
    confirming.value = false
  }

  async function confirm(start: (cfg: S3ImportConfig) => Promise<void>) {
    const cfg: S3ImportConfig = {
      bucket: s3.value.bucket.trim(),
      prefix: s3.value.prefix?.trim() || undefined,
      region: s3.value.region?.trim() || undefined,
      accessKeyId: s3.value.accessKeyId?.trim() || undefined,
      secretAccessKey: s3.value.secretAccessKey || undefined,
      sessionToken: s3.value.sessionToken || undefined,
    }
    confirming.value = false
    await start(cfg)
  }

  return {
    s3, confirming, valid, credsLabel,
    profiles, activeProfile, profileSelectValue, profileSelectLabel, deleteConfirmOpen,
    onProfileChange, saveProfile, deleteProfile,
    review, cancel, confirm,
  }
}
