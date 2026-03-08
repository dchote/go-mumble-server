<template>
  <v-container>
    <BrandCard title-class="text-h5" content-class="pa-3 pa-sm-6">
      <template #header>
        <BackButton fallback="/servers" class="mr-2" />
        <span class="text-h5 header-truncate">{{ server?.name || `Server ${serverId}` }}</span>
      </template>
      <v-alert v-if="error" type="error" density="compact" class="mb-4">{{ error }}</v-alert>

      <v-row>
        <v-col cols="12" md="7">
          <h3 class="text-subtitle-1 font-weight-bold mb-3">Channels</h3>
          <v-progress-linear v-if="channelsLoading" indeterminate class="mb-2" />
          <ChannelTree v-else :channels="channelTree" />
        </v-col>
        <v-col cols="12" md="5">
          <h3 class="text-subtitle-1 font-weight-bold mb-3">Connected Users</h3>
          <v-progress-linear v-if="usersLoading" indeterminate class="mb-2" />
          <p v-else-if="users.length === 0" class="text-body-2">No users connected.</p>
          <v-list v-else density="compact">
            <v-list-item
              v-for="u in users"
              :key="u.session_id || u.id"
              :prepend-avatar="undefined"
              :title="u.name || u.username || 'Unknown'"
            />
          </v-list>
        </v-col>
      </v-row>
    </BrandCard>
  </v-container>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import BrandCard from '@/components/common/BrandCard.vue'
import BackButton from '@/components/common/BackButton.vue'
import ChannelTree from '@/components/ChannelTree.vue'
import api from '@/utils/api'

const route = useRoute()
const server = ref(null)
const channelTree = ref([])
const users = ref([])
const error = ref('')
const channelsLoading = ref(true)
const usersLoading = ref(true)

const serverId = computed(() => route.params.id)

async function loadServer() {
  try {
    const list = await api.get('/servers')
    server.value = list.find((s) => String(s.id) === String(serverId.value)) || { id: serverId.value, name: `Server ${serverId.value}` }
  } catch (e) {
    error.value = e.message || 'Failed to load server'
  }
}

async function loadChannels() {
  channelsLoading.value = true
  try {
    channelTree.value = await api.get(`/servers/${serverId.value}/channels`)
  } catch (e) {
    error.value = e.message || 'Failed to load channels'
  } finally {
    channelsLoading.value = false
  }
}

async function loadUsers() {
  usersLoading.value = true
  try {
    users.value = await api.get(`/servers/${serverId.value}/users`)
  } catch (e) {
    error.value = e.message || 'Failed to load users'
  } finally {
    usersLoading.value = false
  }
}

onMounted(() => {
  loadServer().then(() => {
    loadChannels()
    loadUsers()
  })
})

watch(serverId, () => {
  loadServer()
  loadChannels()
  loadUsers()
})
</script>
