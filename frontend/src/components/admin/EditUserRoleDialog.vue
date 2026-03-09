<template>
  <StandardDialog
    :model-value="modelValue"
    title="Edit user role"
    max-width="400"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="$emit('close')"
  >
    <v-select
      :model-value="user?.role"
      :items="[{ title: 'Admin', value: 'admin' }, { title: 'User', value: 'user' }]"
      label="Role"
      variant="outlined"
      density="compact"
      hide-details="auto"
      autocomplete="off"
      @update:model-value="$emit('update:role', $event)"
    />
    <template #actions>
      <v-spacer />
      <v-btn variant="text" class="mr-2" @click="$emit('update:modelValue', false)">Cancel</v-btn>
      <v-btn color="primary" variant="elevated" :loading="loading" @click="$emit('save')">
        Save
      </v-btn>
    </template>
  </StandardDialog>
</template>

<script setup>
import StandardDialog from '@/components/common/StandardDialog.vue'

defineProps({
  modelValue: {
    type: Boolean,
    default: false,
  },
  user: {
    type: Object,
    default: null,
  },
  loading: {
    type: Boolean,
    default: false,
  },
})
defineEmits(['update:modelValue', 'close', 'update:role', 'save'])
</script>
