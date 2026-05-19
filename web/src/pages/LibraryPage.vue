<script setup lang="ts">
import { ref, watch, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { api, type Email, type Tag } from '@/lib/api'
import { formatBytes, formatDate, shortSHA } from '@/lib/format'
import { useDebounceFn, useStorage } from '@vueuse/core'
import Input from '@/components/ui/Input.vue'
import Button from '@/components/ui/Button.vue'
import Card from '@/components/ui/Card.vue'

const router = useRouter()

const items = ref<Email[]>([])
const total = ref(0)
const limit = ref(50)
const offset = ref(0)
const loading = ref(false)
const q = ref('')
const tag = ref('')
const sort = ref<'sent_at' | 'from_addr' | 'subject' | 'size_bytes'>('sent_at')
const order = ref<'asc' | 'desc'>('desc')
const tags = ref<Tag[]>([])
const sidebarOpen = useStorage('library-sidebar-open', true)

async function load() {
  loading.value = true
  try {
    const r = await api.listEmails({
      q: q.value || undefined,
      tag: tag.value || undefined,
      sort: sort.value,
      order: order.value,
      limit: limit.value,
      offset: offset.value,
    })
    items.value = r.items
    total.value = r.total
  } finally {
    loading.value = false
  }
}

const debouncedLoad = useDebounceFn(load, 250)

watch([q, tag], () => { offset.value = 0; debouncedLoad() })
watch([sort, order, offset], load)

onMounted(async () => {
  tags.value = await api.listTags()
  await load()
})

function setSort(col: typeof sort.value) {
  if (sort.value === col) order.value = order.value === 'asc' ? 'desc' : 'asc'
  else { sort.value = col; order.value = 'desc' }
}

const pageInfo = computed(() => {
  const end = Math.min(offset.value + items.value.length, total.value)
  return total.value === 0 ? '0' : `${offset.value + 1}–${end} of ${total.value}`
})

function open(sha: string) {
  router.push({ name: 'viewer', params: { sha } })
}
</script>

<template>
  <div class="flex gap-6">
    <aside v-if="sidebarOpen" class="w-56 shrink-0 space-y-4">
      <Card class="p-4">
        <div class="flex items-center justify-between mb-3">
          <h3 class="text-xs font-semibold uppercase text-muted-foreground">Tags</h3>
          <button
            class="text-muted-foreground hover:text-foreground text-sm leading-none p-1 -m-1"
            @click="sidebarOpen = false"
            title="Collapse sidebar"
          >‹</button>
        </div>
        <div class="space-y-1">
          <button
            class="w-full text-left px-2 py-1 rounded text-sm hover:bg-accent"
            :class="{ 'bg-accent': tag === '' }"
            @click="tag = ''"
          >All <span class="text-muted-foreground float-right">{{ total }}</span></button>
          <button
            v-for="t in tags"
            :key="t.name"
            class="w-full text-left px-2 py-1 rounded text-sm hover:bg-accent"
            :class="{ 'bg-accent': tag === t.name }"
            @click="tag = t.name"
          >
            {{ t.name }}
            <span class="text-muted-foreground float-right">{{ t.count }}</span>
          </button>
        </div>
      </Card>
    </aside>

    <button
      v-else
      class="self-start text-muted-foreground hover:text-foreground border rounded-md px-2 py-1.5 text-sm leading-none"
      @click="sidebarOpen = true"
      title="Expand sidebar"
    >›</button>

    <section class="flex-1 min-w-0">
      <div class="flex items-center gap-3 mb-4">
        <Input v-model="q" placeholder="Search subject, from, body…" class="max-w-md" />
        <span class="text-sm text-muted-foreground ml-auto">{{ pageInfo }}</span>
      </div>

      <Card class="overflow-hidden">
        <table class="w-full text-sm">
          <thead class="bg-muted/40 text-xs uppercase text-muted-foreground">
            <tr>
              <th class="text-left px-3 py-2 w-10"></th>
              <Th label="Date" col="sent_at" :sort="sort" :order="order" @sort="setSort" />
              <Th label="From" col="from_addr" :sort="sort" :order="order" @sort="setSort" />
              <Th label="Subject" col="subject" :sort="sort" :order="order" @sort="setSort" />
              <Th label="Size" col="size_bytes" :sort="sort" :order="order" @sort="setSort" align="right" />
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading && items.length === 0">
              <td colspan="5" class="px-3 py-6 text-center text-muted-foreground">Loading…</td>
            </tr>
            <tr v-else-if="items.length === 0">
              <td colspan="5" class="px-3 py-6 text-center text-muted-foreground">
                No emails. <RouterLink to="/import" class="underline">Import some?</RouterLink>
              </td>
            </tr>
            <tr
              v-for="e in items"
              :key="e.sha256"
              class="border-t hover:bg-accent/50 cursor-pointer"
              @click="open(e.sha256)"
            >
              <td class="px-3 py-2 text-muted-foreground">
                <span v-if="e.has_attachments" title="Has attachments">📎</span>
              </td>
              <td class="px-3 py-2 whitespace-nowrap text-muted-foreground">{{ formatDate(e.sent_at) }}</td>
              <td class="px-3 py-2 truncate max-w-[18rem]" :title="e.from">{{ e.from }}</td>
              <td class="px-3 py-2 truncate">
                {{ e.subject || '(no subject)' }}
                <span v-for="t in e.tags" :key="t" class="ml-1 inline-block text-[10px] px-1.5 py-0.5 rounded bg-accent text-accent-foreground">{{ t }}</span>
                <span class="ml-2 text-xs text-muted-foreground">{{ shortSHA(e.sha256) }}</span>
              </td>
              <td class="px-3 py-2 text-right whitespace-nowrap text-muted-foreground">{{ formatBytes(e.size_bytes) }}</td>
            </tr>
          </tbody>
        </table>
      </Card>

      <div class="flex justify-between items-center mt-4">
        <Button variant="outline" size="sm" :disabled="offset === 0" @click="offset = Math.max(0, offset - limit)">Prev</Button>
        <span class="text-sm text-muted-foreground">{{ pageInfo }}</span>
        <Button variant="outline" size="sm" :disabled="offset + limit >= total" @click="offset += limit">Next</Button>
      </div>
    </section>
  </div>
</template>

<script lang="ts">
import { defineComponent, h } from 'vue'

export const Th = defineComponent({
  props: {
    label: { type: String, required: true },
    col: { type: String, required: true },
    sort: { type: String, required: true },
    order: { type: String, required: true },
    align: { type: String, default: 'left' },
  },
  emits: ['sort'],
  setup(props, { emit }) {
    return () => {
      const active = props.sort === props.col
      const arrow = active ? (props.order === 'asc' ? ' ↑' : ' ↓') : ''
      return h(
        'th',
        {
          class: ['px-3 py-2 cursor-pointer select-none hover:text-foreground',
            props.align === 'right' ? 'text-right' : 'text-left'],
          onClick: () => emit('sort', props.col),
        },
        props.label + arrow,
      )
    }
  },
})
</script>
