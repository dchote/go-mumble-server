<template>
  <v-container>
    <v-row>
      <v-col cols="12">
        <BrandCard title="Server Status" title-class="text-h6">
          <v-progress-linear v-if="loading" indeterminate class="mb-2" />
          <template v-else>
            <div class="d-flex align-center mb-2">
              <v-icon color="success" class="mr-2">mdi-check-circle</v-icon>
              <span>{{ status?.status === 'ok' ? 'Running' : 'Unknown' }}</span>
            </div>
            <v-divider class="my-2" />
            <div class="text-body-2">
              <p class="mb-1"><strong>Virtual servers:</strong> {{ status?.servers ?? '-' }}</p>
              <p class="mb-1"><strong>Channels:</strong> {{ status?.channels ?? '-' }}</p>
              <p class="mb-1"><strong>Mumble port:</strong> {{ status?.mumble_port ?? '-' }}</p>
              <p class="mb-0"><strong>REST port:</strong> {{ status?.rest_port ?? '-' }}</p>
            </div>
          </template>
        </BrandCard>
      </v-col>
    </v-row>
  </v-container>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import BrandCard from '@/components/common/BrandCard.vue'
import api from '@/utils/api'

const status = ref(null)
const loading = ref(true)

onMounted(async () => {
  try {
    status.value = await api.get('/status')
  } catch {
    status.value = { status: 'error' }
  } finally {
    loading.value = false
  }
})
</script>
