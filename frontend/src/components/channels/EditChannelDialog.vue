<template>
  <StandardDialog
    v-model="model"
    title="Edit channel"
    max-width="500"
    :persistent="true"
    @close="reset"
  >
    <v-form @submit.prevent="handleSubmit">
      <v-alert v-if="error" type="error" density="compact" class="mb-4">{{ error }}</v-alert>
      <v-text-field
        v-model="form.name"
        label="Name"
        variant="outlined"
        density="compact"
        hide-details="auto"
        autocomplete="off"
        class="mb-4"
      />
      <v-textarea
        v-model="form.description"
        label="Description"
        variant="outlined"
        density="compact"
        hide-details="auto"
        rows="2"
        autocomplete="off"
        class="mb-4"
      />
      <v-text-field
        v-model.number="form.position"
        label="Position"
        type="number"
        variant="outlined"
        density="compact"
        hide-details="auto"
        autocomplete="off"
        class="mb-4"
      />
      <v-text-field
        v-model.number="form.max_users"
        label="Max users (0 = unlimited)"
        type="number"
        variant="outlined"
        density="compact"
        hide-details="auto"
        autocomplete="off"
        class="mb-4"
      />
      <v-checkbox
        v-model="form.is_temporary"
        label="Temporary channel"
        hide-details
        density="compact"
        class="mb-4"
      />
    </v-form>
    <template #actions>
      <v-spacer />
      <v-btn variant="text" class="mr-2" @click="model = false">Cancel</v-btn>
      <v-btn color="primary" variant="elevated" :loading="loading" @click="handleSubmit">
        Save
      </v-btn>
    </template>
  </StandardDialog>
</template>

<script setup>
import { ref, watch } from 'vue'
import StandardDialog from '@/components/common/StandardDialog.vue'
import api from '@/utils/api'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  serverId: { type: [String, Number], required: true },
  channel: { type: Object, default: null },
})
const emit = defineEmits(['update:modelValue', 'updated'])

const model = ref(props.modelValue)
watch(() => props.modelValue, (v) => { model.value = v })
watch(model, (v) => emit('update:modelValue', v))

const form = ref({
  name: '',
  description: '',
  position: 0,
  max_users: 0,
  is_temporary: false,
})
const loading = ref(false)
const error = ref('')

watch([() => props.modelValue, () => props.channel], ([open, ch]) => {
  if (open && ch) {
    form.value = {
      name: ch.name || '',
      description: ch.description || '',
      position: ch.position ?? 0,
      max_users: ch.max_users ?? 0,
      is_temporary: ch.is_temporary ?? false,
    }
    error.value = ''
  }
}, { immediate: true })

function reset() {
  error.value = ''
  loading.value = false
}

async function handleSubmit() {
  if (!props.channel) return
  if (!form.value.name?.trim()) {
    error.value = 'Name is required'
    return
  }
  error.value = ''
  loading.value = true
  try {
    await api.patch(`/servers/${props.serverId}/channels/${props.channel.id}`, {
      name: form.value.name.trim(),
      description: form.value.description || '',
      position: form.value.position ?? 0,
      max_users: form.value.max_users ?? 0,
      is_temporary: form.value.is_temporary,
    })
    emit('updated')
    model.value = false
  } catch (e) {
    error.value = e.message || 'Failed to update channel'
  } finally {
    loading.value = false
  }
}
</script>
