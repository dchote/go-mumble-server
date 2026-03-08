<template>
  <v-container>
    <BrandCard content-class="pa-3 pa-sm-6">
      <template #header>
        <span class="text-h5 header-title">Virtual Servers</span>
        <v-spacer />
        <v-btn color="primary" variant="elevated" size="small" @click="showCreate = true">
          <v-icon start size="small">mdi-plus</v-icon>
          Create server
        </v-btn>
      </template>
      <v-alert v-if="error" type="error" density="compact" class="mb-4">{{ error }}</v-alert>
      <v-sheet v-if="loading">
        <v-progress-linear indeterminate color="primary" />
      </v-sheet>
      <div v-else-if="servers.length > 0" class="servers-list">
        <router-link
          v-for="s in servers"
          :key="s.id"
          :to="`/servers/${s.id}`"
          class="server-list-item"
        >
          <v-icon size="small" color="grey" class="mr-2">mdi-server</v-icon>
          <span class="text-body-1 font-weight-medium flex-grow-1">{{ s.name || 'Unnamed' }}</span>
          <span class="text-body-2 text-medium-emphasis">{{ s.host }}:{{ s.port }}</span>
        </router-link>
      </div>
      <p v-else class="text-body-2 text-medium-emphasis pa-4">No virtual servers.</p>

      <CreateServerDialog
        v-model="showCreate"
        :default-host="status?.mumble_port ? '0.0.0.0' : '0.0.0.0'"
        :default-port="status?.mumble_port || 64738"
        @created="onCreated"
      />
      <EditServerDialog
        v-model="showEdit"
        :server="editingServer"
        @updated="onUpdated"
      />
      <StandardDialog
        v-model="showDelete"
        title="Delete server?"
        max-width="400"
        @close="deletingServer = null"
      >
        <p>Are you sure you want to delete "{{ deletingServer?.name || 'this server' }}"?</p>
        <template #actions>
          <v-spacer />
          <v-btn variant="text" class="mr-2" @click="showDelete = false">Cancel</v-btn>
          <v-btn color="error" variant="elevated" :loading="deleteLoading" @click="confirmDelete">Delete</v-btn>
        </template>
      </StandardDialog>
    </BrandCard>
  </v-container>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import BrandCard from '@/components/common/BrandCard.vue'
import StandardDialog from '@/components/common/StandardDialog.vue'
import CreateServerDialog from '@/components/servers/CreateServerDialog.vue'
import EditServerDialog from '@/components/servers/EditServerDialog.vue'
import api from '@/utils/api'

const servers = ref([])
const status = ref(null)
const error = ref('')
const loading = ref(true)
const showCreate = ref(false)
const showEdit = ref(false)
const editingServer = ref(null)
const showDelete = ref(false)
const deletingServer = ref(null)
const deleteLoading = ref(false)

async function loadServers() {
  try {
    servers.value = await api.get('/servers')
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
  } catch (e) {
    error.value = e.message || 'Failed to load servers'
  } finally {
    loading.value = false
  }
})

function onCreated() {
  loadServers()
}

function openEdit(s) {
  editingServer.value = s
  showEdit.value = true
}

function onUpdated() {
  loadServers()
}

function openDeleteConfirm(s) {
  deletingServer.value = s
  showDelete.value = true
}

async function confirmDelete() {
  if (!deletingServer.value) return
  deleteLoading.value = true
  try {
    await api.delete(`/servers/${deletingServer.value.id}`)
    showDelete.value = false
    deletingServer.value = null
    loadServers()
  } catch (e) {
    error.value = e.message || 'Failed to delete server'
  } finally {
    deleteLoading.value = false
  }
}
</script>

<style scoped>
.server-list-item {
  display: flex;
  align-items: center;
  padding: 12px 0;
  text-decoration: none;
  color: inherit;
  border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
  transition: background-color 0.2s;
}

.server-list-item:last-child {
  border-bottom: none;
}

.server-list-item:hover {
  background-color: rgba(var(--v-theme-on-surface), 0.04);
}
</style>
