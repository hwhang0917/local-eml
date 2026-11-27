import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { api, type S3ImportConfig, type S3Profile } from '@/lib/api'

export function emptyS3(): S3ImportConfig {
  return {
    accessKeyId: '', secretAccessKey: '', sessionToken: '', region: '', bucket: '', prefix: '',
    endpoint: '', pathStyle: false,
  }
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
      ? {
          ...emptyS3(),
          accessKeyId: p.access_key_id ?? '',
          region: p.region ?? '',
          bucket: p.bucket,
          prefix: p.prefix ?? '',
          endpoint: p.endpoint ?? '',
          pathStyle: p.path_style ?? false,
        }
      : emptyS3()
  }

  /** Loads a saved profile into the form, or clears it for a new one. */
  function selectProfile(p: S3Profile | null) {
    selectedProfileId.value = p?.id ?? null
    applyProfile(p)
  }

  // Saving a new profile asks for its name in a modal first; saving the
  // loaded one keeps its name and writes straight away.
  const nameDialogOpen = ref(false)
  const saving = ref(false)
  const suggestedName = computed(() => s3.value.bucket.trim())

  async function saveProfile() {
    const existing = activeProfile.value
    if (!existing) {
      nameDialogOpen.value = true
      return
    }
    await persistProfile(existing.name)
  }

  async function saveProfileAs(name: string) {
    if (await persistProfile(name)) nameDialogOpen.value = false
  }

  async function persistProfile(name: string): Promise<boolean> {
    saving.value = true
    try {
      const saved = await api.saveS3Profile({
        name,
        bucket: s3.value.bucket.trim(),
        prefix: s3.value.prefix?.trim() || undefined,
        region: s3.value.region?.trim() || undefined,
        access_key_id: s3.value.accessKeyId?.trim() || undefined,
        endpoint: s3.value.endpoint?.trim() || undefined,
        path_style: !!s3.value.pathStyle,
      })
      await loadProfiles()
      selectedProfileId.value = saved.id
      return true
    } catch (e) {
      toast.error(t('import.s3_profile_save_error'), { description: String(e) })
      return false
    } finally {
      saving.value = false
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

  // A custom endpoint means a non-AWS service, where the SDK has no region
  // to discover, so the server signs with us-east-1 unless one is given.
  const usesEndpoint = computed(() => !!s3.value.endpoint?.trim())

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
      endpoint: s3.value.endpoint?.trim() || undefined,
      pathStyle: !!s3.value.pathStyle,
    }
    confirming.value = false
    await start(cfg)
  }

  return {
    s3, confirming, valid, credsLabel, usesEndpoint,
    profiles, selectedProfileId, activeProfile, deleteConfirmOpen,
    selectProfile, saveProfile, saveProfileAs, deleteProfile,
    nameDialogOpen, saving, suggestedName,
    review, cancel, confirm,
  }
}
