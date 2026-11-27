<script setup lang="ts">
import { computed } from 'vue'
import { buttonClass, type ButtonSize, type ButtonVariant } from '@/lib/utils'

const props = withDefaults(
  defineProps<{
    variant?: ButtonVariant
    size?: ButtonSize
    type?: 'button' | 'submit' | 'reset'
    disabled?: boolean
  }>(),
  { variant: 'default', size: 'md', type: 'button', disabled: false },
)

const cls = computed(() => buttonClass(props.variant, props.size))
</script>

<template>
  <button :type="type" :disabled="disabled" :class="cls">
    <slot />
  </button>
</template>
