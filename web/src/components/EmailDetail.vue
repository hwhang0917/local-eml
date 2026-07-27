<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { Star } from 'lucide-vue-next'
import { api, type Email, type PartsManifest } from '@/lib/api'
import { APP_NAME } from '@/lib/app'
import { formatBytes, formatDateAbsolute } from '@/lib/format'
import Button from '@/components/ui/Button.vue'
import Card from '@/components/ui/Card.vue'

const props = defineProps<{ sha: string }>()
const { t, locale } = useI18n()
const router = useRouter()

const email = ref<Email | null>(null)
const parts = ref<PartsManifest | null>(null)
const tab = ref<'html' | 'text' | 'raw' | 'attachments'>('html')
const textBody = ref('')
const rawBody = ref('')
const showRemote = ref(false)
const error = ref('')
const pendingLink = ref('')
const linkDialog = ref<HTMLDialogElement | null>(null)

const htmlSrc = computed(() =>
  email.value ? api.htmlURL(email.value.sha256, showRemote.value, locale.value) : '')

const tabs = computed(() => [
  { key: 'html' as const, label: t('viewer.tabs.html') },
  { key: 'text' as const, label: t('viewer.tabs.text') },
  { key: 'attachments' as const, label: t('viewer.tabs.attachments') + (parts.value ? ` (${parts.value.attachments.length})` : '') },
  { key: 'raw' as const, label: t('viewer.tabs.raw') },
])

async function load() {
  error.value = ''
  textBody.value = ''
  rawBody.value = ''
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

watch(() => email.value?.subject, (subject) => {
  document.title = subject ? `${subject} — ${APP_NAME}` : APP_NAME
})

function goBack() {
  if (window.history.length > 1) router.back()
  else router.push('/')
}

// The message iframe is sandboxed without allow-scripts, so a link click would
// otherwise navigate the iframe itself and render the remote site inside the
// viewer. The frame is same-origin, so the parent can listen on its document
// and hand external links to a confirmation dialog instead.
function guardIframeLinks(e: Event) {
  const doc = (e.target as HTMLIFrameElement).contentDocument
  doc?.addEventListener('click', onIframeClick)
}

function onIframeClick(e: MouseEvent) {
  const anchor = (e.target as Element | null)?.closest?.('a[href]') as HTMLAnchorElement | null
  if (!anchor) return
  // mailto:/tel: are handed to the OS and never navigate the frame.
  if (anchor.protocol !== 'http:' && anchor.protocol !== 'https:') return
  e.preventDefault()
  pendingLink.value = anchor.href
  linkDialog.value?.showModal()
}

function openPendingLink() {
  window.open(pendingLink.value, '_blank', 'noopener,noreferrer')
  linkDialog.value?.close()
}

async function toggleStar() {
  if (!email.value) return
  const next = !email.value.starred
  email.value.starred = next
  try {
    await api.setStarred(email.value.sha256, next)
  } catch {
    if (email.value) email.value.starred = !next
  }
}
</script>

<template>
  <div v-if="error" class="text-destructive">{{ error }}</div>

  <div v-else-if="email" class="space-y-4">
    <button
      type="button"
      @click="goBack"
      class="inline-flex items-center text-sm text-muted-foreground hover:text-foreground"
    >
      {{ t('viewer.back') }}
    </button>

    <Card class="p-5">
      <div class="mb-3 flex items-start gap-3">
        <h1 class="text-xl font-semibold flex-1 min-w-0">{{ email.subject || t('library.no_subject') }}</h1>
        <button
          type="button"
          :title="email.starred ? t('library.unstar') : t('library.star')"
          :aria-label="email.starred ? t('library.unstar') : t('library.star')"
          :class="['inline-flex items-center justify-center h-8 w-8 rounded-sm hover:bg-accent shrink-0',
            email.starred ? 'text-amber-500' : 'text-muted-foreground hover:text-foreground']"
          @click="toggleStar"
        >
          <Star class="h-5 w-5" :fill="email.starred ? 'currentColor' : 'none'" />
        </button>
      </div>
      <dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
        <dt class="text-muted-foreground">{{ t('viewer.from') }}</dt><dd>{{ email.from }}</dd>
        <dt class="text-muted-foreground">{{ t('viewer.to') }}</dt><dd>{{ email.to.join(', ') }}</dd>
        <dt v-if="email.cc.length" class="text-muted-foreground">{{ t('viewer.cc') }}</dt><dd v-if="email.cc.length">{{ email.cc.join(', ') }}</dd>
        <dt class="text-muted-foreground">{{ t('viewer.date') }}</dt><dd>{{ formatDateAbsolute(email.sent_at) }}</dd>
        <dt class="text-muted-foreground">{{ t('viewer.size') }}</dt><dd>{{ formatBytes(email.size_bytes) }}</dd>
      </dl>
    </Card>

    <div class="flex items-center gap-1 border-b">
      <button v-for="tb in tabs" :key="tb.key" type="button" @click="tab = tb.key"
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
        class="w-full h-[70vh]" referrerpolicy="no-referrer" @load="guardIframeLinks"></iframe>
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

    <dialog ref="linkDialog"
      class="m-auto w-[90vw] max-w-md rounded-lg border bg-background p-0 text-foreground backdrop:bg-black/50">
      <div class="space-y-3 p-5">
        <h2 class="font-semibold">{{ t('viewer.external_link.title') }}</h2>
        <p class="text-sm text-muted-foreground">{{ t('viewer.external_link.body') }}</p>
        <p class="rounded-sm bg-accent p-2 font-mono text-xs break-all">{{ pendingLink }}</p>
        <div class="flex justify-end gap-2">
          <Button variant="ghost" size="sm" @click="linkDialog?.close()">
            {{ t('viewer.external_link.cancel') }}
          </Button>
          <Button size="sm" @click="openPendingLink">
            {{ t('viewer.external_link.open') }}
          </Button>
        </div>
      </div>
    </dialog>
  </div>

  <p v-else class="text-muted-foreground">{{ t('viewer.loading') }}</p>
</template>
