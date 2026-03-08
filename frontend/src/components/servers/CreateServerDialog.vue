<template>
  <StandardDialog
    v-model="model"
    title="Create virtual server"
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
        placeholder="Server"
      />
      <v-text-field
        v-model="form.host"
        label="Host"
        variant="outlined"
        density="compact"
        hide-details="auto"
        autocomplete="off"
        class="mb-4"
        placeholder="0.0.0.0"
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
        Create
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
  defaultHost: { type: String, default: '0.0.0.0' },
  defaultPort: { type: Number, default: 64738 },
})
const emit = defineEmits(['update:modelValue', 'created'])

const model = ref(props.modelValue)
watch(() => props.modelValue, (v) => { model.value = v })
watch(model, (v) => emit('update:modelValue', v))

const form = ref({
  name: 'Server',
  host: props.defaultHost,
  port: props.defaultPort,
  max_users: 100,
  welcome_text: '',
})
const loading = ref(false)
const error = ref('')

watch(() => props.modelValue, (v) => {
  if (v) {
    form.value = {
      name: 'Server',
      host: props.defaultHost,
      port: props.defaultPort,
      max_users: 100,
      welcome_text: '',
    }
    error.value = ''
  }
})

function reset() {
  error.value = ''
  loading.value = false
}

async function handleSubmit() {
  error.value = ''
  loading.value = true
  try {
    const server = await api.post('/servers', form.value)
    emit('created', server)
    model.value = false
  } catch (e) {
    error.value = e.message || 'Failed to create server'
  } finally {
    loading.value = false
  }
}
</script>
