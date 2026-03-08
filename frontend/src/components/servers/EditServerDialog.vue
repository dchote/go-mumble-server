<template>
  <StandardDialog
    v-model="model"
    title="Edit virtual server"
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
      <v-text-field
        v-model="form.host"
        label="Host"
        variant="outlined"
        density="compact"
        hide-details="auto"
        autocomplete="off"
        class="mb-4"
      />
      <v-text-field
        v-model.number="form.port"
        label="Port"
        type="number"
        variant="outlined"
        density="compact"
        hide-details="auto"
        autocomplete="off"
        class="mb-4"
      />
      <v-text-field
        v-model.number="form.max_users"
        label="Max users"
        type="number"
        variant="outlined"
        density="compact"
        hide-details="auto"
        autocomplete="off"
        class="mb-4"
      />
      <v-textarea
        v-model="form.welcome_text"
        label="Welcome text"
        variant="outlined"
        density="compact"
        hide-details="auto"
        rows="3"
        autocomplete="off"
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
  server: { type: Object, default: null },
})
const emit = defineEmits(['update:modelValue', 'updated'])

const model = ref(props.modelValue)
watch(() => props.modelValue, (v) => { model.value = v })
watch(model, (v) => emit('update:modelValue', v))

const form = ref({
  name: '',
  host: '',
  port: 64738,
  max_users: 100,
  welcome_text: '',
})
const loading = ref(false)
const error = ref('')

watch([() => props.modelValue, () => props.server], ([open, s]) => {
  if (open && s) {
    form.value = {
      name: s.name || '',
      host: s.host || '0.0.0.0',
      port: s.port || 64738,
      max_users: s.max_users || 100,
      welcome_text: s.welcome_text || '',
    }
    error.value = ''
  }
}, { immediate: true })

function reset() {
  error.value = ''
  loading.value = false
}

async function handleSubmit() {
  if (!props.server) return
  error.value = ''
  loading.value = true
  try {
    const updated = await api.patch(`/servers/${props.server.id}`, form.value)
    emit('updated', updated)
    model.value = false
  } catch (e) {
    error.value = e.message || 'Failed to update server'
  } finally {
    loading.value = false
  }
}
</script>
