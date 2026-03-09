<template>
  <v-container>
    <v-row>
      <v-col cols="12">
        <StandardCard title="Server Status" title-class="text-h6">
          <template #header>
            <span class="text-h6 header-truncate">Server Status</span>
            <v-spacer />
            <div v-if="!loading" class="d-flex align-center">
              <v-icon
                :color="status?.status === 'ok' ? 'success' : 'default'"
                icon="mdi-check-circle"
                size="small"
                class="mr-1"
              />
              <span class="text-body-2">{{ status?.status === 'ok' ? 'Running' : 'Unknown' }}</span>
            </div>
          </template>
          <v-progress-linear v-if="loading" indeterminate class="mb-2" />
          <template v-else>
            <ServerStatusInfo
              :virtual-server-count="status?.servers"
              :channel-count="status?.channels"
              :connected-users="connectedUsersCount"
              :show-virtual-server-count="true"
            />

            <div v-if="connectedUsers.length > 0" class="mt-4">
              <h3 class="text-subtitle-1 font-weight-bold mb-2">Connected clients</h3>
              <v-table density="comfortable">
                <thead>
                  <tr>
                    <th>Username</th>
                    <th>IP Address</th>
                    <th>Ping</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="u in connectedUsers" :key="u.session_id">
                    <td>{{ u.name || u.username || 'Unknown' }}</td>
                    <td>{{ u.address || u.ip || '-' }}</td>
                    <td>{{ u.ping != null ? `${u.ping} ms` : '-' }}</td>
                  </tr>
                </tbody>
              </v-table>
            </div>
          </template>
        </StandardCard>
      </v-col>
    </v-row>
  </v-container>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import StandardCard from '@/components/common/StandardCard.vue'
import ServerStatusInfo from '@/components/servers/ServerStatusInfo.vue'
import api from '@/utils/api'

const status = ref(null)
const connectedUsers = ref([])
const loading = ref(true)

const connectedUsersCount = computed(() => {
  return connectedUsers.value?.length ?? 0
})

onMounted(async () => {
  try {
    const [st, users] = await Promise.all([
      api.get('/status'),
      api.get('/servers/1/users').catch(() => []),
    ])
    status.value = st
    connectedUsers.value = Array.isArray(users) ? users : []
  } catch {
    status.value = { status: 'error' }
    connectedUsers.value = []
  } finally {
    loading.value = false
  }
})
</script>
