<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { Bookmark, BookmarkPlus, Check, X } from 'lucide-vue-next'
import {
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuPortal,
  DropdownMenuRoot,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from 'reka-ui'
import { api, type SavedSearch } from '@/lib/api'
import PromptDialog from '@/components/ui/PromptDialog.vue'

// Saved searches: a name for a search-box query. Since every filter button
// writes into the box, the query is the whole view, and applying one is just
// putting its text back.

const props = defineProps<{ query: string }>()
const emit = defineEmits<{ apply: [query: string] }>()
const { t } = useI18n()

const list = ref<SavedSearch[]>([])
const saving = ref(false)
const promptOpen = ref(false)
// The dialog opens once the menu has finished closing: the menu hands focus
// back to its button as it closes, which would otherwise pull focus out of
// the dialog's name field.
let promptPending = false
function askName() {
  promptPending = true
}
function onMenuCloseFocus(e: Event) {
  if (!promptPending) return
  promptPending = false
  e.preventDefault()
  promptOpen.value = true
}

const norm = (s: string) => s.trim().replace(/\s+/g, ' ')
const active = computed(() => list.value.find((s) => norm(s.query) === norm(props.query)))

onMounted(async () => {
  try {
    list.value = await api.listSavedSearches()
  } catch {
    // The menu still offers to save; a failed load shouldn't block the library.
  }
})

async function store(next: SavedSearch[]): Promise<boolean> {
  saving.value = true
  try {
    list.value = await api.saveSavedSearches(next)
    return true
  } catch (err) {
    toast.error(t('saved_search.save_error'), { description: String(err) })
    return false
  } finally {
    saving.value = false
  }
}

async function save(name: string) {
  const query = norm(props.query)
  // Saving under an existing name replaces that search, in place.
  const i = list.value.findIndex((s) => s.name.toLowerCase() === name.toLowerCase())
  const next = [...list.value]
  if (i >= 0) next[i] = { name, query }
  else next.push({ name, query })
  if (await store(next)) {
    promptOpen.value = false
    toast.success(t('saved_search.saved', { name }))
  }
}

async function remove(s: SavedSearch) {
  await store(list.value.filter((x) => x !== s))
}
</script>

<template>
  <DropdownMenuRoot>
    <DropdownMenuTrigger
      data-testid="saved-searches"
      :class="['inline-flex h-8 items-center gap-2 rounded-sm border px-3 text-sm',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
        active ? 'border-primary text-primary bg-pearl' : 'border-hairline bg-pearl text-muted-foreground hover:text-foreground']"
    >
      <Bookmark class="h-4 w-4" :fill="active ? 'currentColor' : 'none'" />
      <span class="max-w-40 truncate">{{ active ? active.name : t('saved_search.button') }}</span>
    </DropdownMenuTrigger>
    <DropdownMenuPortal>
      <DropdownMenuContent
        :side-offset="6"
        align="start"
        class="z-50 w-80 rounded-lg border border-hairline bg-background p-1 shadow-lg"
        @close-auto-focus="onMenuCloseFocus"
      >
        <p v-if="list.length === 0" class="px-2 py-1.5 text-xs text-muted-foreground">
          {{ t('saved_search.empty') }}
        </p>
        <DropdownMenuItem
          v-for="s in list"
          :key="s.name"
          class="group flex cursor-default select-none items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-none
            data-[highlighted]:bg-accent"
          @select="emit('apply', s.query)"
        >
          <span class="inline-flex h-4 w-4 shrink-0 items-center justify-center">
            <Check v-if="s === active" class="h-4 w-4" />
          </span>
          <span class="grid min-w-0 flex-1">
            <span class="truncate font-medium">{{ s.name }}</span>
            <code class="truncate font-mono text-xs text-muted-foreground">{{ s.query }}</code>
          </span>
          <button
            type="button"
            :title="t('saved_search.delete', { name: s.name })"
            :aria-label="t('saved_search.delete', { name: s.name })"
            :disabled="saving"
            class="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-sm text-muted-foreground
              hover:bg-background hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            @click.stop.prevent="remove(s)"
            @pointerdown.stop
          >
            <X class="h-3.5 w-3.5" />
          </button>
        </DropdownMenuItem>
        <DropdownMenuSeparator class="my-1 h-px bg-hairline" />
        <DropdownMenuItem
          :disabled="!query.trim()"
          data-testid="save-search"
          class="flex cursor-default select-none items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-none
            data-[highlighted]:bg-accent data-[disabled]:opacity-50"
          @select="askName"
        >
          <BookmarkPlus class="h-4 w-4" />
          <span>{{ query.trim() ? t('saved_search.save') : t('saved_search.save_disabled') }}</span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenuPortal>
  </DropdownMenuRoot>
  <PromptDialog
    v-model:open="promptOpen"
    :title="t('saved_search.dialog_title')"
    :description="t('saved_search.dialog_body', { query: norm(query) })"
    :label="t('saved_search.dialog_label')"
    :confirm-label="t('saved_search.dialog_confirm')"
    :cancel-label="t('saved_search.dialog_cancel')"
    :initial-value="active?.name ?? ''"
    :placeholder="t('saved_search.dialog_placeholder')"
    :busy="saving"
    @submit="save"
  />
</template>
