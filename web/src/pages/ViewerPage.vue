<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { api, type Email, type PartsManifest } from '@/lib/api'
import { formatBytes, formatDate } from '@/lib/format'
import Card from '@/components/ui/Card.vue'
import {
  TagsInput,
  TagsInputInput,
  TagsInputItem,
  TagsInputItemText,
  TagsInputItemDelete,
} from '@/components/ui/tags-input'

const props = defineProps<{ sha: string }>()
const { t } = useI18n()

const email = ref<Email | null>(null)
const parts = ref<PartsManifest | null>(null)
const tab = ref<'html' | 'text' | 'raw' | 'attachments'>('html')
const textBody = ref('')
const rawBody = ref('')
const showRemote = ref(false)
const error = ref('')

const htmlSrc = computed(() => email.value ? api.htmlURL(email.value.sha256, showRemote.value) : '')

const tabs = computed(() => [
  { key: 'html' as const, label: t('viewer.tabs.html') },
  { key: 'text' as const, label: t('viewer.tabs.text') },
  { key: 'attachments' as const, label: t('viewer.tabs.attachments') + (parts.value ? ` (${parts.value.attachments.length})` : '') },
  { key: 'raw' as const, label: t('viewer.tabs.raw') },
])

async function load() {
  error.value = ''
  try {
    const [e, p] = await Promise.all([api.getEmail(props.sha), api.getParts(props.sha)])
    email.value = e
    parts.value = p
    if (p.has_html) tab.value = 'html'
    else if (p.has_text) tab.value = 'text'
    else if (p.attachments.length > 0) tab.value = 'attachments'
    else tab.value = 'raw'
  } catch (err) {
    error.value = String(err)
  }
}

watch(() => props.sha, load, { immediate: true })

watch([tab, () => email.value?.sha256], async ([tb, sha]) => {
  if (!sha) return
  if (tb === 'text' && !textBody.value) textBody.value = await api.getText(sha)
  if (tb === 'raw' && !rawBody.value) rawBody.value = await api.getRaw(sha)
})

async function onTagAdd(tag: string) {
  if (!email.value) return
  const trimmed = tag.trim()
  if (!trimmed) return
  try {
    await api.addTag(email.value.sha256, trimmed)
    email.value.tags = [...email.value.tags, trimmed].sort()
  } catch (e) {
    toast.error(t('viewer.tag_invalid'), { description: String(e) })
  }
}

async function onTagRemove(tag: string) {
  if (!email.value) return
  try {
    await api.removeTag(email.value.sha256, tag)
    email.value.tags = email.value.tags.filter((x) => x !== tag)
  } catch (e) {
    toast.error(t('viewer.tag_remove_failed'), { description: String(e) })
  }
}
</script>

<template>
  <div v-if="error" class="text-destructive">{{ error }}</div>

  <div v-else-if="email" class="space-y-4">
    <RouterLink to="/" class="text-sm text-muted-foreground hover:underline">{{ t('viewer.back') }}</RouterLink>

    <Card class="p-5">
      <h1 class="text-xl font-semibold mb-3">{{ email.subject || t('library.no_subject') }}</h1>
      <dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
        <dt class="text-muted-foreground">{{ t('viewer.from') }}</dt><dd>{{ email.from }}</dd>
        <dt class="text-muted-foreground">{{ t('viewer.to') }}</dt><dd>{{ email.to.join(', ') }}</dd>
        <dt v-if="email.cc.length" class="text-muted-foreground">{{ t('viewer.cc') }}</dt><dd v-if="email.cc.length">{{ email.cc.join(', ') }}</dd>
        <dt class="text-muted-foreground">{{ t('viewer.date') }}</dt><dd>{{ formatDate(email.sent_at) }}</dd>
        <dt class="text-muted-foreground">{{ t('viewer.size') }}</dt><dd>{{ formatBytes(email.size_bytes) }}</dd>
      </dl>

      <div class="mt-4">
        <div class="text-xs uppercase tracking-wide text-muted-foreground mb-1.5">{{ t('viewer.tags') }}</div>
        <TagsInput
          :model-value="email.tags"
          :add-on-paste="true"
          :delimiter="/[,]/"
          @add="(p) => onTagAdd(p.value)"
          @remove="(p) => onTagRemove(p.value)"
        >
          <TagsInputItem v-for="tg in email.tags" :key="tg" :value="tg">
            <TagsInputItemText>{{ tg }}</TagsInputItemText>
            <TagsInputItemDelete />
          </TagsInputItem>
          <TagsInputInput :placeholder="t('viewer.add_tag')" />
        </TagsInput>
      </div>
    </Card>

    <div class="flex items-center gap-1 border-b">
      <button v-for="tb in tabs" :key="tb.key" @click="tab = tb.key"
        :class="['px-3 py-2 text-sm border-b-2 -mb-px',
          tab === tb.key ? 'border-foreground' : 'border-transparent text-muted-foreground hover:text-foreground']">
        {{ tb.label }}
      </button>
      <div class="ml-auto flex items-center gap-2 pb-1" v-if="tab === 'html'">
        <label class="text-xs text-muted-foreground flex items-center gap-1 cursor-pointer">
          <input type="checkbox" v-model="showRemote" /> {{ t('viewer.load_remote') }}
        </label>
      </div>
    </div>

    <div v-if="tab === 'html'" class="border rounded-lg overflow-hidden bg-white">
      <iframe v-if="parts?.has_html" :src="htmlSrc" sandbox="allow-same-origin"
        class="w-full h-[70vh]" referrerpolicy="no-referrer"></iframe>
      <p v-else class="p-6 text-muted-foreground">{{ t('viewer.no_html') }}</p>
    </div>

    <Card v-else-if="tab === 'text'" class="p-4">
      <pre v-if="parts?.has_text" class="whitespace-pre-wrap font-mono text-sm">{{ textBody }}</pre>
      <p v-else class="text-muted-foreground">{{ t('viewer.no_text') }}</p>
    </Card>

    <Card v-else-if="tab === 'raw'" class="p-4">
      <pre class="whitespace-pre-wrap font-mono text-xs max-h-[70vh] overflow-auto">{{ rawBody }}</pre>
    </Card>

    <Card v-else-if="tab === 'attachments'" class="p-4">
      <ul v-if="parts && parts.attachments.length" class="divide-y">
        <li v-for="a in parts.attachments" :key="a.index" class="py-2 flex items-center gap-3">
          <span class="font-medium flex-1">{{ a.filename || `attachment-${a.index}` }}</span>
          <span class="text-xs text-muted-foreground">{{ a.content_type }}</span>
          <span class="text-xs text-muted-foreground">{{ formatBytes(a.size) }}</span>
          <a :href="api.attachmentURL(email.sha256, a.index)" target="_blank"
            class="text-sm underline">{{ t('viewer.download') }}</a>
        </li>
      </ul>
      <p v-else class="text-muted-foreground">{{ t('viewer.no_attachments') }}</p>
    </Card>
  </div>

  <p v-else class="text-muted-foreground">{{ t('viewer.loading') }}</p>
</template>
