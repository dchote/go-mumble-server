<template>
  <v-container>
    <StandardCard>
      <template #header>
        <span class="text-h5 header-title">Virtual Servers</span>
        <v-spacer />
        <v-btn
          color="primary"
          variant="elevated"
          size="small"
          @click="showCreate = true"
        >
          <v-icon start size="small">mdi-plus</v-icon>
          Create server
        </v-btn>
      </template>
      <v-alert v-if="error" type="error" density="compact" class="mb-4">{{ error }}</v-alert>
      <v-sheet v-if="loading">
        <v-progress-linear indeterminate color="primary" />
      </v-sheet>
      <v-data-table
        v-else-if="servers.length > 0"
        :items="servers"
        :headers="headers"
        density="comfortable"
        :items-per-page="50"
        class="servers-table"
      >
        <template #bottom></template>
        <template #item.name="{ item }">
          <router-link :to="`/servers/${item.id}`" class="text-primary text-decoration-none font-weight-medium">
            {{ item.name || 'Unnamed' }}
          </router-link>
        </template>
        <template #item.channelCount="{ item }">
          {{ getServerStats(item.id).channelCount ?? '-' }}
        </template>
        <template #item.connectedUsers="{ item }">
          {{ getServerStats(item.id).connectedUsers ?? '-' }}
        </template>
      </v-data-table>
      <p v-else class="text-body-2 text-medium-emphasis">No virtual servers.</p>

      <CreateServerDialog
        v-model="showCreate"
        default-host="0.0.0.0"
        :default-port="status?.mumble_port || 64738"
        @created="onCreated"
      />
    </StandardCard>
  </v-container>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import StandardCard from '@/components/common/StandardCard.vue'
import CreateServerDialog from '@/components/servers/CreateServerDialog.vue'
import api from '@/utils/api'

const servers = ref([])
const status = ref(null)
const serverStats = ref({})
const error = ref('')
const loading = ref(true)
const showCreate = ref(false)

const headers = [
  { title: 'Name', key: 'name', sortable: true },
  { title: 'Channel Count', key: 'channelCount' },
  { title: 'Max Users', key: 'max_users' },
  { title: 'Connected Users', key: 'connectedUsers' },
]

function countChannels(channels) {
  if (!channels?.length) return 0
  return channels.reduce((acc, ch) => 1 + acc + countChannels(ch.children || []), 0)
}

function getServerStats(serverId) {
  return serverStats.value[serverId] ?? {}
}

async function loadServerStats(serverList) {
  const stats = {}
  await Promise.all(
    serverList.map(async (s) => {
      try {
        const [channels, users] = await Promise.all([
          api.get(`/servers/${s.id}/channels`).catch(() => []),
          api.get(`/servers/${s.id}/users`).catch(() => []),
        ])
        stats[s.id] = {
          channelCount: Array.isArray(channels) ? countChannels(channels) : 0,
          connectedUsers: Array.isArray(users) ? users.length : 0,
        }
      } catch {
        stats[s.id] = {}
      }
    })
  )
  serverStats.value = stats
}

async function loadServers() {
  try {
    const list = await api.get('/servers')
    servers.value = list
    await loadServerStats(list)
  } catch (e) {
    error.value = e.message || 'Failed to load servers'
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  loading.value = true
  try {
    const [srvList, st] = await Promise.all([api.get('/servers'), api.get('/status').catch(() => null)])
    servers.value = srvList
    status.value = st
    await loadServerStats(srvList)
  } catch (e) {
    error.value = e.message || 'Failed to load servers'
  } finally {
    loading.value = false
  }
})

function onCreated() {
  loadServers()
}
</script>

