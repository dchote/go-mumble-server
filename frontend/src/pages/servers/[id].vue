<template>
  <v-container>
    <BrandCard title-class="text-h5" content-class="pa-3 pa-sm-6">
      <template #header>
        <BackButton fallback="/servers" class="mr-2" />
        <span class="text-h5 header-truncate">{{ server?.name || `Server ${serverId}` }}</span>
        <v-spacer />
        <v-menu location="bottom end">
          <template #activator="{ props: menuProps }">
            <v-btn v-bind="menuProps" icon="mdi-dots-vertical" variant="text" size="small" />
          </template>
          <v-list density="compact">
            <v-list-item prepend-icon="mdi-pencil" title="Edit server" @click="openEdit" />
            <v-list-item prepend-icon="mdi-delete" title="Delete" @click="showDeleteConfirm = true" />
          </v-list>
        </v-menu>
      </template>
      <v-alert v-if="error" type="error" density="compact" class="mb-4">{{ error }}</v-alert>

      <div class="mb-4">
        <p class="text-body-2 text-medium-emphasis mb-0">
          <strong>Address:</strong> {{ server?.host || '-' }}:{{ server?.port || '-' }}
          <span class="ml-4"><strong>Max users:</strong> {{ server?.max_users ?? '-' }}</span>
        </p>
      </div>

      <div class="section-header d-flex align-center mb-2">
        <h3 class="text-subtitle-1 font-weight-bold mb-0">Channels</h3>
        <v-spacer />
        <v-btn
          v-if="serverId"
          size="small"
          variant="elevated"
          color="primary"
          @click="openCreateChannel(0)"
        >
          <v-icon start size="small">mdi-plus</v-icon>
          Add Channel
        </v-btn>
      </div>
      <v-progress-linear v-if="channelsLoading" indeterminate class="mb-2" />
      <ChannelTree
        v-else
        :channels="channelTree"
        :server-id="serverId"
        :users="users"
        @create-sub="openCreateChannel"
        @edit="openEditChannel"
        @acl="openACL"
        @delete="openDeleteChannel"
      />

      <div class="section-header d-flex align-center mt-6 mb-2">
        <h3 class="text-subtitle-1 font-weight-bold mb-0">Bans</h3>
        <v-spacer />
        <v-btn size="small" variant="outlined" @click="showAddBan = true">Add ban</v-btn>
      </div>
      <v-progress-linear v-if="bansLoading" indeterminate class="mb-2" />
      <p v-else-if="bans.length === 0" class="text-body-2 mb-2">No bans.</p>
      <v-table v-else density="compact" class="mb-4">
        <thead>
          <tr>
            <th>Address / Hash</th>
            <th>Reason</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="b in bans" :key="b.id">
            <td>{{ b.address || b.hash || '-' }}</td>
            <td>{{ b.reason || '-' }}</td>
            <td>
              <v-btn icon="mdi-delete" variant="text" size="x-small" @click="deleteBan(b)" />
            </td>
          </tr>
        </tbody>
      </v-table>

      <div class="section-header d-flex align-center mb-2">
        <h3 class="text-subtitle-1 font-weight-bold mb-0">Registered users</h3>
        <v-spacer />
        <v-btn size="small" variant="outlined" @click="showAddRegUser = true">Register user</v-btn>
      </div>
      <v-progress-linear v-if="regUsersLoading" indeterminate class="mb-2" />
      <p v-else-if="regUsers.length === 0" class="text-body-2 mb-2">No registered users.</p>
      <v-table v-else density="compact" class="mb-4">
        <thead>
          <tr>
            <th>Name</th>
            <th>User ID</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in regUsers" :key="u.id">
            <td>{{ u.name }}</td>
            <td>{{ u.user_id }}</td>
            <td>
              <v-btn icon="mdi-delete" variant="text" size="x-small" @click="deleteRegUser(u)" />
            </td>
          </tr>
        </tbody>
      </v-table>

      <EditServerDialog v-model="showEditDialog" :server="server" @updated="loadServer" />
      <StandardDialog v-model="showDeleteConfirm" title="Delete server?" max-width="400" @close="showDeleteConfirm = false">
        <p>Are you sure you want to delete "{{ server?.name || 'this server' }}"?</p>
        <template #actions>
          <v-spacer />
          <v-btn variant="text" class="mr-2" @click="showDeleteConfirm = false">Cancel</v-btn>
          <v-btn color="error" variant="elevated" :loading="deleteServerLoading" @click="confirmDeleteServer">Delete</v-btn>
        </template>
      </StandardDialog>
      <CreateChannelDialog
        v-model="showCreateChannelDialog"
        :server-id="serverId"
        :channels="channelTree"
        :parent-id="createChannelParentId"
        @created="loadChannels"
      />
      <EditChannelDialog
        v-model="showEditChannelDialog"
        :server-id="serverId"
        :channel="editingChannel"
        @updated="loadChannels"
      />
      <ACLDialog v-model="showACLDialog" :server-id="serverId" :channel="aclChannel" />
      <StandardDialog v-model="showDeleteChannelDialog" title="Delete channel?" max-width="400" @close="deletingChannel = null">
        <p>Are you sure you want to delete "{{ deletingChannel?.name || 'this channel' }}"? It must have no subchannels.</p>
        <template #actions>
          <v-spacer />
          <v-btn variant="text" class="mr-2" @click="showDeleteChannelDialog = false">Cancel</v-btn>
          <v-btn color="error" variant="elevated" :loading="deleteChannelLoading" @click="confirmDeleteChannel">Delete</v-btn>
        </template>
      </StandardDialog>

      <StandardDialog v-model="showAddRegUser" title="Register user" max-width="400" @close="newRegUser = { name: '', password: '' }">
        <v-text-field
          v-model="newRegUser.name"
          label="Name"
          variant="outlined"
          density="compact"
          hide-details="auto"
          autocomplete="off"
          class="mb-4"
        />
        <v-text-field
          v-model="newRegUser.password"
          label="Password"
          type="password"
          variant="outlined"
          density="compact"
          hide-details="auto"
          autocomplete="new-password"
        />
        <template #actions>
          <v-spacer />
          <v-btn variant="text" class="mr-2" @click="showAddRegUser = false">Cancel</v-btn>
          <v-btn color="primary" variant="elevated" :loading="addRegUserLoading" @click="submitRegUser">Register</v-btn>
        </template>
      </StandardDialog>

      <StandardDialog v-model="showAddBan" title="Add ban" max-width="450" @close="newBan = { address: '', hash: '', reason: '' }">
        <v-text-field
          v-model="newBan.address"
          label="IP address"
          variant="outlined"
          density="compact"
          hide-details="auto"
          autocomplete="off"
          class="mb-4"
          placeholder="192.168.1.1"
        />
        <v-text-field
          v-model="newBan.hash"
          label="Certificate hash (hex)"
          variant="outlined"
          density="compact"
          hide-details="auto"
          autocomplete="off"
          class="mb-4"
          placeholder="Optional"
        />
        <v-text-field
          v-model="newBan.reason"
          label="Reason"
          variant="outlined"
          density="compact"
          hide-details="auto"
          autocomplete="off"
        />
        <template #actions>
          <v-spacer />
          <v-btn variant="text" class="mr-2" @click="showAddBan = false">Cancel</v-btn>
          <v-btn color="primary" variant="elevated" :loading="addBanLoading" @click="submitBan">Add</v-btn>
        </template>
      </StandardDialog>
    </BrandCard>
  </v-container>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import BrandCard from '@/components/common/BrandCard.vue'
import BackButton from '@/components/common/BackButton.vue'
import StandardDialog from '@/components/common/StandardDialog.vue'
import ChannelTree from '@/components/ChannelTree.vue'
import CreateChannelDialog from '@/components/channels/CreateChannelDialog.vue'
import EditChannelDialog from '@/components/channels/EditChannelDialog.vue'
import EditServerDialog from '@/components/servers/EditServerDialog.vue'
import ACLDialog from '@/components/channels/ACLDialog.vue'
import api from '@/utils/api'

const route = useRoute()
const router = useRouter()
const server = ref(null)
const channelTree = ref([])
const users = ref([])
const error = ref('')
const channelsLoading = ref(true)
const usersLoading = ref(true)
const bansLoading = ref(true)
const bans = ref([])
const showAddBan = ref(false)
const newBan = ref({ address: '', hash: '', reason: '' })
const addBanLoading = ref(false)
const regUsersLoading = ref(true)
const regUsers = ref([])
const showAddRegUser = ref(false)
const newRegUser = ref({ name: '', password: '' })
const addRegUserLoading = ref(false)
const showEditDialog = ref(false)
const showDeleteConfirm = ref(false)

const serverId = computed(() => route.params.id)
const showCreateChannelDialog = ref(false)
const createChannelParentId = ref(0)
const showEditChannelDialog = ref(false)
const editingChannel = ref(null)
const showACLDialog = ref(false)
const aclChannel = ref(null)
const showDeleteChannelDialog = ref(false)
const deletingChannel = ref(null)
const deleteChannelLoading = ref(false)
const deleteServerLoading = ref(false)

function openEdit() {
  showEditDialog.value = true
}

function openCreateChannel(parent) {
  createChannelParentId.value = parent?.id ?? 0
  showCreateChannelDialog.value = true
}
function openEditChannel(ch) {
  editingChannel.value = ch
  showEditChannelDialog.value = true
}
function openACL(ch) {
  aclChannel.value = ch
  showACLDialog.value = true
}
function openDeleteChannel(ch) {
  deletingChannel.value = ch
  showDeleteChannelDialog.value = true
}
async function confirmDeleteServer() {
  if (!server.value?.id) return
  deleteServerLoading.value = true
  try {
    await api.delete(`/servers/${server.value.id}`)
    showDeleteConfirm.value = false
    router.push('/servers')
  } catch (e) {
    error.value = e.message || 'Failed to delete server'
  } finally {
    deleteServerLoading.value = false
  }
}

async function confirmDeleteChannel() {
  if (!deletingChannel.value) return
  deleteChannelLoading.value = true
  try {
    await api.delete(`/servers/${serverId.value}/channels/${deletingChannel.value.id}`)
    showDeleteChannelDialog.value = false
    deletingChannel.value = null
    loadChannels()
  } catch (e) {
    error.value = e.message || 'Failed to delete channel'
  } finally {
    deleteChannelLoading.value = false
  }
}

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

async function loadBans() {
  bansLoading.value = true
  try {
    bans.value = await api.get(`/servers/${serverId.value}/bans`)
  } catch (e) {
    error.value = e.message || 'Failed to load bans'
  } finally {
    bansLoading.value = false
  }
}

async function submitBan() {
  if (!newBan.value.address && !newBan.value.hash) {
    error.value = 'Address or hash required'
    return
  }
  addBanLoading.value = true
  try {
    await api.post(`/servers/${serverId.value}/bans`, {
      address: newBan.value.address || undefined,
      hash: newBan.value.hash || undefined,
      reason: newBan.value.reason || '',
    })
    showAddBan.value = false
    newBan.value = { address: '', hash: '', reason: '' }
    loadBans()
  } catch (e) {
    error.value = e.message || 'Failed to add ban'
  } finally {
    addBanLoading.value = false
  }
}

async function deleteBan(b) {
  try {
    await api.delete(`/servers/${serverId.value}/bans/${b.id}`)
    loadBans()
  } catch (e) {
    error.value = e.message || 'Failed to delete ban'
  }
}

async function loadRegUsers() {
  regUsersLoading.value = true
  try {
    regUsers.value = await api.get(`/servers/${serverId.value}/registered-users`)
  } catch (e) {
    error.value = e.message || 'Failed to load registered users'
  } finally {
    regUsersLoading.value = false
  }
}

async function submitRegUser() {
  if (!newRegUser.value.name?.trim()) {
    error.value = 'Name is required'
    return
  }
  addRegUserLoading.value = true
  try {
    await api.post(`/servers/${serverId.value}/registered-users`, {
      name: newRegUser.value.name.trim(),
      password: newRegUser.value.password || undefined,
    })
    showAddRegUser.value = false
    newRegUser.value = { name: '', password: '' }
    loadRegUsers()
  } catch (e) {
    error.value = e.message || 'Failed to register user'
  } finally {
    addRegUserLoading.value = false
  }
}

async function deleteRegUser(u) {
  try {
    await api.delete(`/servers/${serverId.value}/registered-users/${u.user_id}`)
    loadRegUsers()
  } catch (e) {
    error.value = e.message || 'Failed to delete user'
  }
}

onMounted(() => {
  loadServer().then(() => {
    loadChannels()
    loadUsers()
    loadBans()
    loadRegUsers()
  })
})

watch(serverId, () => {
  loadServer()
  loadChannels()
  loadUsers()
  loadBans()
  loadRegUsers()
})
</script>
