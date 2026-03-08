<template>
  <v-container>
    <BrandCard title="Virtual Servers" title-class="text-h5" content-class="pa-3 pa-sm-6">
      <v-alert v-if="error" type="error" density="compact" class="mb-4">{{ error }}</v-alert>
      <v-row v-if="servers.length > 0">
        <v-col
          v-for="s in servers"
          :key="s.id"
          cols="12"
          sm="6"
          md="4"
        >
          <v-card variant="outlined" :to="`/servers/${s.id}`" class="fill-height">
            <v-card-title class="d-flex align-center">
              <v-icon class="mr-2">mdi-server</v-icon>
              {{ s.name || 'Unnamed' }}
            </v-card-title>
            <v-card-text>
              <p class="text-body-2 mb-1">
                <strong>Address:</strong> {{ s.host }}:{{ s.port }}
              </p>
              <p class="text-body-2 mb-0">
                <strong>Max users:</strong> {{ s.max_users }}
              </p>
            </v-card-text>
            <v-card-actions>
              <v-btn variant="text" size="small" :to="`/servers/${s.id}`">
                View channels
              </v-btn>
            </v-card-actions>
          </v-card>
        </v-col>
      </v-row>
      <p v-else-if="!loading" class="text-body-2">No virtual servers.</p>
      <v-progress-linear v-if="loading" indeterminate class="mb-2" />
    </BrandCard>
  </v-container>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import BrandCard from '@/components/common/BrandCard.vue'
import api from '@/utils/api'

const servers = ref([])
const error = ref('')
const loading = ref(true)

onMounted(async () => {
  try {
    servers.value = await api.get('/servers')
  } catch (e) {
    error.value = e.message || 'Failed to load servers'
  } finally {
    loading.value = false
  }
})
</script>
