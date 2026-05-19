<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, type Tag } from '@/lib/api'
import Card from '@/components/ui/Card.vue'

const tags = ref<Tag[]>([])
const tagsLoading = ref(true)
const origin = window.location.origin

onMounted(async () => {
  try { tags.value = await api.listTags() } finally { tagsLoading.value = false }
})
</script>

<template>
  <div class="grid gap-6 md:grid-cols-2">
    <Card class="p-5">
      <h2 class="font-semibold mb-3">About</h2>
      <dl class="text-sm space-y-1">
        <div class="flex justify-between"><dt class="text-muted-foreground">Server</dt><dd><code>{{ origin }}</code></dd></div>
        <div class="flex justify-between"><dt class="text-muted-foreground">Build</dt><dd>dev</dd></div>
      </dl>
      <p class="mt-3 text-xs text-muted-foreground">
        Data lives in <code>~/.local-eml/</code> (or <code>%USERPROFILE%\.local-eml\</code> on Windows).
      </p>
    </Card>

    <Card class="p-5">
      <h2 class="font-semibold mb-3">Tags</h2>
      <p v-if="tagsLoading" class="text-sm text-muted-foreground">Loading…</p>
      <p v-else-if="tags.length === 0" class="text-sm text-muted-foreground">No tags yet.</p>
      <ul v-else class="text-sm divide-y">
        <li v-for="t in tags" :key="t.name" class="py-1.5 flex justify-between">
          <span>{{ t.name }}</span>
          <span class="text-muted-foreground">{{ t.count }}</span>
        </li>
      </ul>
    </Card>
  </div>
</template>
