<template>
  <v-container>
    <StandardCard title="Settings" title-class="text-h5">
      <h3 class="text-subtitle-1 font-weight-bold mb-3">Instance Configuration</h3>
      <v-form @submit.prevent="handleSave">
        <v-alert v-if="error" type="error" density="compact" class="mb-4">{{ error }}</v-alert>
        <v-alert v-if="success" type="success" density="compact" class="mb-4">Settings saved.</v-alert>
        <v-select
          v-model="form.security_mode"
          label="Security mode"
          :items="[
            { title: 'Legacy', value: 'legacy' },
            { title: 'Secure', value: 'secure' },
          ]"
          variant="outlined"
          density="compact"
          hide-details="auto"
          class="mb-4"
        />
        <v-text-field
          v-model="form.host"
          label="Bind address"
          variant="outlined"
          density="compact"
          hide-details="auto"
          autocomplete="off"
          class="mb-4"
        />
        <v-text-field
          v-model.number="form.mumble_port"
          label="Mumble port"
          type="number"
          variant="outlined"
          density="compact"
          hide-details="auto"
          autocomplete="off"
          class="mb-4"
        />
        <v-text-field
          v-model.number="form.rest_port"
          label="REST API port"
          type="number"
          variant="outlined"
          density="compact"
          hide-details="auto"
          autocomplete="off"
          class="mb-4"
        />
        <v-checkbox
          v-model="form.bonjour"
          label="Enable Bonjour/mDNS discovery"
          hide-details
          density="compact"
          class="mb-4"
        />
        <v-text-field
          v-model="form.register_name"
          label="Bonjour register name"
          variant="outlined"
          density="compact"
          hide-details="auto"
          autocomplete="off"
          class="mb-4"
          placeholder="go-mumble-server"
        />
        <div class="d-flex justify-end mt-4">
          <v-btn type="submit" color="primary" variant="elevated" :loading="loading">Save</v-btn>
        </div>
      </v-form>
    </StandardCard>
  </v-container>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import StandardCard from '@/components/common/StandardCard.vue'
import api from '@/utils/api'

const form = ref({
  security_mode: 'legacy',
  host: '0.0.0.0',
  mumble_port: 64738,
  rest_port: 64730,
  bonjour: false,
  register_name: 'go-mumble-server',
})
const loading = ref(false)
const error = ref('')
const success = ref(false)

onMounted(async () => {
  try {
    const data = await api.get('/meta/config')
    form.value = {
      security_mode: data.security_mode || 'legacy',
      host: data.host || '0.0.0.0',
      mumble_port: data.mumble_port ?? 64738,
      rest_port: data.rest_port ?? 64730,
      bonjour: data.bonjour ?? false,
      register_name: data.register_name || 'go-mumble-server',
    }
  } catch (e) {
    error.value = e.message || 'Failed to load config'
  }
})

async function handleSave() {
  error.value = ''
  success.value = false
  loading.value = true
  try {
    await api.patch('/meta/config', form.value)
    success.value = true
  } catch (e) {
    error.value = e.message || 'Failed to save settings'
  } finally {
    loading.value = false
  }
}
</script>
