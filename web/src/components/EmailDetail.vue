<script setup lang="ts">
import { ref, watch, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useStorage } from '@vueuse/core'
import { ChevronDown, ChevronLeft, ChevronRight, CircleHelp, FileWarning, Star } from 'lucide-vue-next'
import { api, type Email, type PartsManifest } from '@/lib/api'
import { useCategories } from '@/composables/useCategories'
import { useListContext, type Neighbors } from '@/composables/useListContext'
import { hasModifier, isTypingTarget } from '@/lib/keys'
import { APP_NAME } from '@/lib/app'
import { formatBytes, formatDate, formatDateAbsolute, senderName, shortSHA } from '@/lib/format'
import Button from '@/components/ui/Button.vue'
import CategoryDot from '@/components/ui/CategoryDot.vue'
import CategoryMenu, { type CategoryOption } from '@/components/ui/CategoryMenu.vue'
import Card from '@/components/ui/Card.vue'

const props = defineProps<{ sha: string }>()
const { t, locale } = useI18n()
const router = useRouter()

const email = ref<Email | null>(null)
const parts = ref<PartsManifest | null>(null)
const thread = ref<Email[]>([])
const tab = ref<'html' | 'text' | 'raw' | 'attachments'>('html')
const textBody = ref('')
const rawBody = ref('')
const showRemote = ref(false)
const error = ref('')
// Fold state survives navigation — collapsing the metadata once should keep
// the reading space maximised across the whole session.
const showMeta = useStorage('viewer-show-meta', true)
const showThread = useStorage('viewer-show-thread', true)
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
    const e = await api.getEmail(props.sha)
    email.value = e
    if (!e.thread_id) {
      thread.value = []
    } else if (!thread.value.some((m) => m.sha256 === e.sha256)) {
      // Moving within the already-loaded conversation keeps the card as-is —
      // clearing and refetching made it unmount and pop back, jolting the page.
      // Not awaited: the card appearing late beats the whole viewer waiting.
      api.getThread(props.sha)
        .then((r) => { thread.value = r.items })
        .catch(() => { thread.value = [] })
    }
    // With no file on disk every body endpoint 404s, so stop here and let the
    // template offer to clear the dangling row.
    if (e.blob_missing) {
      parts.value = null
      // Arriving from a raw/text tab would otherwise keep that panel mounted
      // and fire a body fetch that can only 404.
      tab.value = 'html'
      return
    }
    const p = await api.getParts(props.sha)
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

// The button promises "back to library", so that is where it must go. history
// back is only taken when the previous entry actually is the library — it
// restores the filters and scroll position — otherwise (direct link, arrived
// from import/settings) we navigate there outright.
function goBack() {
  const prev = router.options.history.state.back
  if (typeof prev === 'string' && router.resolve(prev).name === 'library') router.back()
  else router.push({ name: 'library' })
}

// Prev/next inside the list the user came from. The context lives in
// sessionStorage; with none (direct link) the nav simply doesn't render.
const listCtx = useListContext()
const nav = ref<Neighbors | null>(null)
watch(() => props.sha, async (sha) => {
  // Keep the stale nav visible while the new one loads — nulling it made the
  // prev/next controls blink out on every navigation.
  try {
    nav.value = await listCtx.neighbors(sha)
  } catch {
    nav.value = null
  }
}, { immediate: true })

function goSibling(target: Email | null, delta: number) {
  if (!target) return
  listCtx.shift(delta)
  // replace, so Back still returns to the library in one step
  router.replace({ name: 'viewer', params: { sha: target.sha256 } })
}

function onKeydown(e: KeyboardEvent) {
  if (hasModifier(e) || isTypingTarget(e) || !nav.value) return
  if (e.key === 'ArrowLeft') goSibling(nav.value.prev, -1)
  else if (e.key === 'ArrowRight') goSibling(nav.value.next, 1)
}
onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))

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

const repairing = ref(false)
const indexDialog = ref<HTMLDialogElement | null>(null)

// flush: 'post' — the dialog element only exists once the email has rendered.
watch(
  () => email.value?.not_indexed,
  (orphaned) => {
    if (orphaned) indexDialog.value?.showModal()
  },
  { flush: 'post' },
)

async function deleteDangling() {
  repairing.value = true
  try {
    await api.deleteEmail(props.sha)
    router.push('/')
  } catch (err) {
    error.value = String(err)
  } finally {
    repairing.value = false
  }
}

async function indexOrphan() {
  repairing.value = true
  try {
    await api.indexEmail(props.sha)
    indexDialog.value?.close()
    await load()
  } catch (err) {
    error.value = String(err)
  } finally {
    repairing.value = false
  }
}

const { categories, byId, labelFor, load: loadCategories } = useCategories()
onMounted(loadCategories)

const assignOptions = computed<CategoryOption[]>(() => [
  { value: 'none', label: t('library.category_none') },
  ...categories.value.map((c) => ({ value: String(c.id), label: labelFor(c), color: c.color })),
])
const currentCategory = computed(() =>
  email.value?.category_id ? byId.value.get(email.value.category_id) : undefined,
)

async function setCategory(value: string) {
  if (!email.value) return
  const next = value === 'none' ? null : Number(value)
  const prev = email.value.category_id
  email.value.category_id = next ?? undefined
  try {
    await api.setCategory(email.value.sha256, next)
  } catch {
    if (email.value) email.value.category_id = prev
  }
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
    <!-- min-h-7 matches the nav buttons, so the row keeps its height when the
         prev/next nav hides (direct links, thread jumps outside the list ctx). -->
    <div class="flex min-h-7 items-center gap-2">
      <button
        type="button"
        @click="goBack"
        class="inline-flex items-center text-sm text-muted-foreground hover:text-foreground"
      >
        {{ t('viewer.back') }}
      </button>
      <div v-if="nav" class="ml-auto flex items-center gap-1 text-sm text-muted-foreground">
        <span class="mr-1 tabular-nums">{{ t('viewer.position', { index: nav.index + 1, total: nav.total }) }}</span>
        <button
          type="button"
          :disabled="!nav.prev"
          :title="t('viewer.prev')"
          :aria-label="t('viewer.prev')"
          class="inline-flex h-7 w-7 items-center justify-center rounded-sm hover:bg-accent hover:text-foreground
            disabled:pointer-events-none disabled:opacity-40"
          @click="goSibling(nav.prev, -1)"
        >
          <ChevronLeft class="h-4 w-4" />
        </button>
        <button
          type="button"
          :disabled="!nav.next"
          :title="t('viewer.next')"
          :aria-label="t('viewer.next')"
          class="inline-flex h-7 w-7 items-center justify-center rounded-sm hover:bg-accent hover:text-foreground
            disabled:pointer-events-none disabled:opacity-40"
          @click="goSibling(nav.next, 1)"
        >
          <ChevronRight class="h-4 w-4" />
        </button>
      </div>
    </div>

    <Card class="p-5">
      <div :class="['flex items-start gap-3', showMeta ? 'mb-3' : '']">
        <h1 class="text-xl font-semibold flex-1 min-w-0">
          {{ email.subject || t('library.no_subject') }}
          <!-- One inline-flex wrapper so the icon centres on the hash text
               rather than the h1's much taller line box. -->
          <span class="ml-1.5 inline-flex items-center gap-1.5 align-middle whitespace-nowrap text-xs font-normal text-muted-foreground">
            <span :title="email.sha256">({{ shortSHA(email.sha256) }})</span>
            <span
              :title="t('viewer.sha_help')"
              :aria-label="t('viewer.sha_help')"
              class="inline-flex"
            >
              <CircleHelp class="h-3.5 w-3.5" />
            </span>
          </span>
        </h1>
        <CategoryMenu
          v-if="!email.blob_missing"
          :model-value="email.category_id ? String(email.category_id) : 'none'"
          :options="assignOptions"
        @select="setCategory"
        >
          <template #trigger>
            <button
              type="button"
              class="inline-flex h-8 shrink-0 items-center gap-2 rounded-sm px-2 text-sm hover:bg-accent"
            >
              <CategoryDot :color="currentCategory?.color" />
              <span :class="currentCategory ? '' : 'text-muted-foreground'">
                {{ currentCategory ? labelFor(currentCategory) : t('library.category_none') }}
              </span>
            </button>
          </template>
        </CategoryMenu>
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
        <button
          type="button"
          :title="t('viewer.toggle_details')"
          :aria-label="t('viewer.toggle_details')"
          :aria-expanded="showMeta"
          class="inline-flex items-center justify-center h-8 w-8 rounded-sm hover:bg-accent shrink-0
            text-muted-foreground hover:text-foreground"
          @click="showMeta = !showMeta"
        >
          <ChevronDown :class="['h-4 w-4 transition-transform', showMeta ? '' : '-rotate-90']" />
        </button>
      </div>
      <dl v-show="showMeta" class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
        <dt class="text-muted-foreground">{{ t('viewer.from') }}</dt><dd>{{ email.from }}</dd>
        <dt class="text-muted-foreground">{{ t('viewer.to') }}</dt><dd>{{ email.to.join(', ') }}</dd>
        <dt v-if="email.cc.length" class="text-muted-foreground">{{ t('viewer.cc') }}</dt><dd v-if="email.cc.length">{{ email.cc.join(', ') }}</dd>
        <dt class="text-muted-foreground">{{ t('viewer.date') }}</dt><dd>{{ formatDateAbsolute(email.sent_at) }}</dd>
        <dt class="text-muted-foreground">{{ t('viewer.size') }}</dt><dd>{{ formatBytes(email.size_bytes) }}</dd>
      </dl>
    </Card>

    <Card v-if="thread.length > 1" class="p-4">
      <h2 class="text-sm font-semibold">
        <button
          type="button"
          :aria-expanded="showThread"
          class="inline-flex items-center gap-1.5 rounded-sm hover:text-foreground"
          @click="showThread = !showThread"
        >
          <ChevronDown :class="['h-4 w-4 transition-transform', showThread ? '' : '-rotate-90']" />
          {{ t('viewer.conversation', { count: thread.length }) }}
        </button>
      </h2>
      <ol v-show="showThread" class="mt-2 space-y-0.5 text-sm">
        <li v-for="m in thread" :key="m.sha256">
          <RouterLink
            :to="{ name: 'viewer', params: { sha: m.sha256 } }"
            :aria-current="m.sha256 === email.sha256 ? 'page' : undefined"
            :class="['flex items-baseline gap-3 rounded-sm px-2 py-1 hover:bg-accent/60',
              m.sha256 === email.sha256 ? 'bg-accent' : '']"
          >
            <span class="w-28 shrink-0 truncate" :title="m.from">{{ senderName(m.from) || m.from }}</span>
            <span class="min-w-0 flex-1 truncate" :class="m.sha256 === email.sha256 ? '' : 'text-muted-foreground'">
              {{ m.subject || t('library.no_subject') }}
            </span>
            <span class="shrink-0 whitespace-nowrap text-xs text-muted-foreground">{{ formatDate(m.sent_at) }}</span>
          </RouterLink>
        </li>
      </ol>
    </Card>

    <Card v-if="email.blob_missing" class="p-6">
      <div class="flex items-start gap-4">
        <FileWarning class="mt-0.5 h-6 w-6 shrink-0 text-destructive" />
        <div class="flex-1 space-y-2">
          <h2 class="font-semibold">{{ t('viewer.missing_file.title') }}</h2>
          <p class="text-sm text-muted-foreground">{{ t('viewer.missing_file.body') }}</p>
          <p class="rounded-sm bg-accent p-2 font-mono text-xs break-all">
            {{ email.filename || `${email.sha256}.eml` }}
          </p>
          <p class="text-sm">{{ t('viewer.missing_file.question') }}</p>
          <div class="flex gap-2 pt-1">
            <Button variant="destructive" size="sm" :disabled="repairing" @click="deleteDangling">
              {{ t('viewer.missing_file.confirm') }}
            </Button>
            <Button variant="ghost" size="sm" :disabled="repairing" @click="goBack">
              {{ t('viewer.missing_file.decline') }}
            </Button>
          </div>
        </div>
      </div>
    </Card>

    <div v-else class="flex items-center gap-1 border-b" role="tablist">
      <button v-for="tb in tabs" :key="tb.key" type="button" role="tab"
        :aria-selected="tab === tb.key" @click="tab = tb.key"
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

    <div v-if="!email.blob_missing && tab === 'html'" role="tabpanel" class="border rounded-lg overflow-hidden bg-white">
      <iframe v-if="parts?.has_html" :src="htmlSrc" sandbox="allow-same-origin"
        class="w-full h-[70vh]" referrerpolicy="no-referrer" @load="guardIframeLinks"></iframe>
      <p v-else class="p-6 text-muted-foreground">{{ t('viewer.no_html') }}</p>
    </div>

    <Card v-else-if="tab === 'text'" role="tabpanel" class="p-4">
      <pre v-if="parts?.has_text" class="whitespace-pre-wrap font-mono text-sm">{{ textBody }}</pre>
      <p v-else class="text-muted-foreground">{{ t('viewer.no_text') }}</p>
    </Card>

    <Card v-else-if="tab === 'raw'" role="tabpanel" class="p-4">
      <pre class="whitespace-pre-wrap font-mono text-xs max-h-[70vh] overflow-auto">{{ rawBody }}</pre>
    </Card>

    <Card v-else-if="tab === 'attachments'" role="tabpanel" class="p-4">
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

    <dialog ref="indexDialog" aria-labelledby="index-dialog-title"
      class="m-auto w-[90vw] max-w-md rounded-lg border bg-background p-0 text-foreground backdrop:bg-black/50">
      <div class="space-y-3 p-5">
        <h2 id="index-dialog-title" class="font-semibold">{{ t('viewer.not_indexed.title') }}</h2>
        <p class="text-sm text-muted-foreground">{{ t('viewer.not_indexed.body') }}</p>
        <div class="flex justify-end gap-2">
          <Button variant="ghost" size="sm" :disabled="repairing" @click="indexDialog?.close()">
            {{ t('viewer.not_indexed.decline') }}
          </Button>
          <Button size="sm" :disabled="repairing" @click="indexOrphan">
            {{ t('viewer.not_indexed.confirm') }}
          </Button>
        </div>
      </div>
    </dialog>

    <dialog ref="linkDialog" aria-labelledby="link-dialog-title"
      class="m-auto w-[90vw] max-w-md rounded-lg border bg-background p-0 text-foreground backdrop:bg-black/50">
      <div class="space-y-3 p-5">
        <h2 id="link-dialog-title" class="font-semibold">{{ t('viewer.external_link.title') }}</h2>
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
