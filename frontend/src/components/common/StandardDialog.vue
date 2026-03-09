<template>
  <v-dialog
    :model-value="modelValue"
    :max-width="maxWidth"
    :fullscreen="fullscreen"
    :persistent="persistent"
    :scrim="scrim"
    @update:model-value="handleUpdate"
  >
    <v-card
      class="brand-dialog-card"
      :class="{
        'dialog-card-layout': hasActions,
        'dialog-card-layout--fullscreen': hasActions && fullscreen,
      }"
    >
      <template v-if="showHeader">
        <v-card-title class="d-flex align-center justify-space-between">
          <slot name="header">
            <span class="text-h6">{{ title }}</span>
          </slot>
          <v-btn
            v-if="showClose && !hasActions"
            icon="mdi-close"
            variant="text"
            size="small"
            :disabled="closeDisabled"
            @click="handleClose"
          />
        </v-card-title>
        <v-divider />
      </template>

      <v-card-text
        :class="[
          contentPadding,
          { 'dialog-card-content': hasActions },
        ]"
      >
        <slot />
      </v-card-text>

      <template v-if="hasActions">
        <v-divider />
        <v-card-actions class="dialog-card-actions">
          <slot name="actions" />
        </v-card-actions>
      </template>
    </v-card>
  </v-dialog>
</template>

<script setup>
import { computed, useSlots } from 'vue'

defineProps({
  modelValue: {
    type: Boolean,
    default: false,
  },
  title: {
    type: String,
    default: '',
  },
  maxWidth: {
    type: [String, Number],
    default: '600',
  },
  fullscreen: {
    type: Boolean,
    default: false,
  },
  persistent: {
    type: Boolean,
    default: true,
  },
  showClose: {
    type: Boolean,
    default: true,
  },
  closeDisabled: {
    type: Boolean,
    default: false,
  },
  showHeader: {
    type: Boolean,
    default: true,
  },
  scrim: {
    type: [Boolean, String],
    default: true,
  },
  contentPadding: {
    type: String,
    default: '',
  },
})

const emit = defineEmits(['update:modelValue', 'close'])
const slots = useSlots()
const hasActions = computed(() => !!slots.actions)

function handleUpdate(value) {
  emit('update:modelValue', value)
  if (!value) emit('close')
}

function handleClose() {
  emit('update:modelValue', false)
  emit('close')
}
</script>
