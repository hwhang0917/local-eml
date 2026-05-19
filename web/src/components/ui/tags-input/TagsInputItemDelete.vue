<script setup lang="ts">
import { TagsInputItemDelete } from 'reka-ui'
import { X } from 'lucide-vue-next'
</script>

<template>
  <TagsInputItemDelete
    class="rounded-full p-0.5 text-muted-foreground hover:bg-foreground/10 hover:text-foreground"
  >
    <X class="h-3 w-3" />
  </TagsInputItemDelete>
</template>
