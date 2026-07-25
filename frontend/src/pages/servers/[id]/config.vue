<template>
  <v-container class="py-4 py-sm-6">
    <StandardCard title-class="text-h5">
      <template #header>
        <BackButton :fallback="`/servers/${serverId}`" class="mr-2" />
        <span class="text-h5 header-truncate">Server configuration</span>
      </template>
      <h3 class="text-subtitle-1 font-weight-bold mb-4">Server Configuration</h3>
      <v-form @submit.prevent="handleSave">
        <v-alert v-if="error" type="error" density="compact" class="mb-4">{{ error }}</v-alert>
        <v-alert v-if="success" type="success" density="compact" class="mb-4">Settings saved.</v-alert>
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
        <v-text-field
          v-model.number="form.max_text_message_length"
          label="Max text message length (bytes, 0 = unlimited)"
          type="number"
          variant="outlined"
          density="compact"
          hide-details="auto"
          autocomplete="off"
          class="mb-4"
        />
        <v-text-field
          v-model.number="form.max_image_message_length"
          label="Max image message length (bytes, 0 = unlimited)"
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
        <v-checkbox
          v-model="form.allow_recording"
          label="Allow recording (clients that start recording are disconnected when disabled)"
          hide-details
          density="compact"
          class="mb-4"
        />
        <div class="section-header mt-6 mb-2">
          <h3 class="text-subtitle-1 font-weight-bold mb-0">Voice path debugging</h3>
        </div>
        <v-checkbox
          v-model="form.voice_debug"
          label="Enable voice path debugging (verbose logs for UDP/TCP tunnel diagnosis)"
          hide-details
          density="compact"
          class="mb-4"
        />
        <div class="d-flex justify-end mt-4">
          <v-btn type="submit" color="primary" variant="elevated" :loading="loading">Save</v-btn>
        </div>
      </v-form>
    </StandardCard>
  </v-container>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import StandardCard from '@/components/common/StandardCard.vue'
import BackButton from '@/components/common/BackButton.vue'
import api from '@/utils/api'

const route = useRoute()
const serverId = computed(() => route.params.id)

const form = ref({
  name: '',
  host: '0.0.0.0',
  port: 64738,
  max_users: 100,
  max_bandwidth: 72000,
  welcome_text: '',
  default_channel: 0,
  cert_required: false,
  voice_debug: false,
  allow_recording: true,
  max_text_message_length: 5000,
  max_image_message_length: 131072,
  channel_nesting_limit: 10,
  channel_count_limit: 1000,
})
const loading = ref(false)
const error = ref('')
const success = ref(false)

onMounted(loadConfig)

async function loadConfig() {
  try {
    const [serverRes, configRes] = await Promise.all([
      api.get(`/servers/${serverId.value}`),
      api.get(`/servers/${serverId.value}/config`),
    ])
    form.value = {
      name: serverRes.name || '',
      host: serverRes.host || '0.0.0.0',
      port: serverRes.port ?? 64738,
      max_users: configRes.max_users ?? 100,
      max_bandwidth: configRes.max_bandwidth ?? 72000,
      welcome_text: configRes.welcome_text || '',
      default_channel: configRes.default_channel ?? 0,
      cert_required: configRes.cert_required ?? false,
      voice_debug: configRes.voice_debug ?? false,
      allow_recording: configRes.allow_recording ?? true,
      max_text_message_length: configRes.max_text_message_length ?? 5000,
      max_image_message_length: configRes.max_image_message_length ?? 131072,
      channel_nesting_limit: configRes.channel_nesting_limit ?? 10,
      channel_count_limit: configRes.channel_count_limit ?? 1000,
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
    await Promise.all([
      api.patch(`/servers/${serverId.value}`, {
        name: form.value.name,
        host: form.value.host,
        port: form.value.port,
        max_users: form.value.max_users,
        welcome_text: form.value.welcome_text,
      }),
      api.patch(`/servers/${serverId.value}/config`, {
        max_users: form.value.max_users,
        max_bandwidth: form.value.max_bandwidth,
        welcome_text: form.value.welcome_text,
        default_channel: form.value.default_channel,
        cert_required: form.value.cert_required,
        voice_debug: form.value.voice_debug,
        allow_recording: form.value.allow_recording,
        max_text_message_length: form.value.max_text_message_length,
        max_image_message_length: form.value.max_image_message_length,
        channel_nesting_limit: form.value.channel_nesting_limit,
        channel_count_limit: form.value.channel_count_limit,
      }),
    ])
    success.value = true
  } catch (e) {
    error.value = e.message || 'Failed to save settings'
  } finally {
    loading.value = false
  }
}
</script>
