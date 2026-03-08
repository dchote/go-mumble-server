<template>
  <v-container>
    <BrandCard title-class="text-h5" content-class="pa-4">
      <template #header>
        <BackButton :fallback="`/servers/${serverId}`" class="mr-2" />
        <span class="text-h5 header-truncate">Server configuration</span>
      </template>
      <v-card variant="flat" class="mb-4">
        <v-card-title class="pa-4 pb-0">Server Configuration</v-card-title>
        <v-card-text class="pa-4 pt-4">
          <v-form @submit.prevent="handleSave">
            <v-alert v-if="error" type="error" density="compact" class="mb-4">{{ error }}</v-alert>
            <v-alert v-if="success" type="success" density="compact" class="mb-4">Settings saved.</v-alert>
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
            <v-text-field
              v-model.number="form.max_bandwidth"
              label="Max bandwidth (bits/sec)"
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
            <v-text-field
              v-model.number="form.default_channel"
              label="Default channel ID"
              type="number"
              variant="outlined"
              density="compact"
              hide-details="auto"
              autocomplete="off"
              class="mb-4"
            />
            <v-checkbox
              v-model="form.cert_required"
              label="Require client certificate"
              hide-details
              density="compact"
              class="mb-4"
            />
            <v-text-field
              v-model.number="form.channel_nesting_limit"
              label="Channel nesting limit"
              type="number"
              variant="outlined"
              density="compact"
              hide-details="auto"
              autocomplete="off"
              class="mb-4"
            />
            <v-text-field
              v-model.number="form.channel_count_limit"
              label="Channel count limit"
              type="number"
              variant="outlined"
              density="compact"
              hide-details="auto"
              autocomplete="off"
              class="mb-4"
            />
            <div class="d-flex justify-end mt-4">
              <v-btn type="submit" color="primary" variant="elevated" :loading="loading">Save</v-btn>
            </div>
          </v-form>
        </v-card-text>
      </v-card>
    </BrandCard>
  </v-container>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import BrandCard from '@/components/common/BrandCard.vue'
import BackButton from '@/components/common/BackButton.vue'
import api from '@/utils/api'

const route = useRoute()
const serverId = computed(() => route.params.id)

const form = ref({
  max_users: 100,
  max_bandwidth: 72000,
  welcome_text: '',
  default_channel: 0,
  cert_required: false,
  channel_nesting_limit: 10,
  channel_count_limit: 1000,
})
const loading = ref(false)
const error = ref('')
const success = ref(false)

onMounted(loadConfig)

async function loadConfig() {
  try {
    const data = await api.get(`/servers/${serverId.value}/config`)
    form.value = {
      max_users: data.max_users ?? 100,
      max_bandwidth: data.max_bandwidth ?? 72000,
      welcome_text: data.welcome_text || '',
      default_channel: data.default_channel ?? 0,
      cert_required: data.cert_required ?? false,
      channel_nesting_limit: data.channel_nesting_limit ?? 10,
      channel_count_limit: data.channel_count_limit ?? 1000,
    }
  } catch (e) {
    error.value = e.message || 'Failed to load config'
  }
}

async function handleSave() {
  error.value = ''
  success.value = false
  loading.value = true
  try {
    await api.patch(`/servers/${serverId.value}/config`, form.value)
    success.value = true
  } catch (e) {
    error.value = e.message || 'Failed to save settings'
  } finally {
    loading.value = false
  }
}
</script>
