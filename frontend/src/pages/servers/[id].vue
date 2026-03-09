<template>
  <v-container>
    <StandardCard title-class="text-h5">
      <template #header>
        <BackButton fallback="/servers" class="mr-2" />
        <span class="text-h5 header-truncate">{{ server?.name || `Server ${serverId}` }}</span>
        <v-spacer />
        <v-menu v-if="isAdmin" location="bottom end">
          <template #activator="{ props: menuProps }">
            <v-btn v-bind="menuProps" icon="mdi-dots-vertical" variant="text" size="small" />
          </template>
          <v-list density="compact">
            <v-list-item prepend-icon="mdi-pencil" title="Edit server" @click="openEdit" />
            <v-tooltip location="top" text="This has not been tested yet">
              <template #activator="{ props: tooltipProps }">
                <v-list-item
                  v-bind="tooltipProps"
                  prepend-icon="mdi-delete"
                  title="Delete server"
                  @click="showDeleteConfirm = true"
                />
              </template>
            </v-tooltip>
          </v-list>
        </v-menu>
      </template>
      <v-alert v-if="error" type="error" density="compact" class="mb-4">{{ error }}</v-alert>

      <div class="mb-4">
        <ServerStatusInfo
          :channel-count="totalChannelCount"
          :max-users="server?.max_users"
          :connected-users="users?.length ?? 0"
        />
      </div>

      <div class="section-header d-flex align-center mb-2">
        <h3 class="text-subtitle-1 font-weight-bold mb-0">Channels</h3>
        <v-spacer />
        <v-btn
          v-if="serverId && isAdmin"
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
        :can-manage-users="isAdmin"
        :show-sensitive-user-data="isAdmin"
        @create-sub="openCreateChannel"
        @edit="openEditChannel"
        @acl="openACL"
        @delete="openDeleteChannel"
        @user-action="handleUserAction"
      />

      <div v-if="isAdmin" class="section-header d-flex align-center mt-6 mb-2">
        <h3 class="text-subtitle-1 font-weight-bold mb-0">Bans</h3>
        <v-spacer />
        <v-btn size="small" variant="outlined" @click="showAddBan = true">Add ban</v-btn>
      </div>
      <v-progress-linear v-if="isAdmin && bansLoading" indeterminate class="mb-2" />
      <p v-else-if="isAdmin && bans.length === 0" class="text-body-2 mb-2">No bans.</p>
      <v-table v-else-if="isAdmin" density="comfortable" class="mb-4">
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

      <div v-if="isAdmin" class="section-header d-flex align-center mb-2">
        <h3 class="text-subtitle-1 font-weight-bold mb-0">Registered users</h3>
        <v-spacer />
        <v-btn size="small" variant="outlined" @click="showAddRegUser = true">Register user</v-btn>
      </div>
      <v-progress-linear v-if="isAdmin && regUsersLoading" indeterminate class="mb-2" />
      <p v-else-if="isAdmin && regUsers.length === 0" class="text-body-2 mb-2">No registered users.</p>
      <v-table v-else-if="isAdmin" density="comfortable" class="mb-4">
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
      <StandardDialog v-model="showMuteDialog" title="Mute user?" max-width="400" @close="muteTarget = null">
        <p>{{ muteTarget?.mute ? `Unmute ${muteTarget?.name || 'this user'}?` : `Mute ${muteTarget?.name || 'this user'}? They will not be able to speak until unmuted.` }}</p>
        <template #actions>
          <v-spacer />
          <v-btn variant="text" class="mr-2" @click="showMuteDialog = false">Cancel</v-btn>
          <v-btn color="primary" variant="elevated" :loading="userActionLoading" @click="confirmMute">
            {{ muteTarget?.mute ? 'Unmute' : 'Mute' }}
          </v-btn>
        </template>
      </StandardDialog>
      <StandardDialog v-model="showKickDialog" title="Kick user?" max-width="400" @close="kickTarget = null">
        <p class="mb-4">Kick {{ kickTarget?.name || 'this user' }}? They can reconnect.</p>
        <v-text-field
          v-model="kickReason"
          label="Reason (optional)"
          variant="outlined"
          density="compact"
          hide-details="auto"
          autocomplete="off"
          class="mb-2"
        />
        <template #actions>
          <v-spacer />
          <v-btn variant="text" class="mr-2" @click="showKickDialog = false">Cancel</v-btn>
          <v-btn color="primary" variant="elevated" :loading="userActionLoading" @click="confirmKick">Kick</v-btn>
        </template>
      </StandardDialog>
      <StandardDialog v-model="showBanDialog" title="Ban user?" max-width="400" @close="banTarget = null">
        <p class="mb-4">Ban {{ banTarget?.name || 'this user' }}? They will be kicked and blocked from reconnecting.</p>
        <v-text-field
          v-model="banReason"
          label="Reason (optional)"
          variant="outlined"
          density="compact"
          hide-details="auto"
          autocomplete="off"
          class="mb-2"
        />
        <template #actions>
          <v-spacer />
          <v-btn variant="text" class="mr-2" @click="showBanDialog = false">Cancel</v-btn>
          <v-btn color="error" variant="elevated" :loading="userActionLoading" @click="confirmBan">Ban</v-btn>
        </template>
      </StandardDialog>
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
    </StandardCard>
  </v-container>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useStore } from 'vuex'
import StandardCard from '@/components/common/StandardCard.vue'
import BackButton from '@/components/common/BackButton.vue'
import StandardDialog from '@/components/common/StandardDialog.vue'
import ChannelTree from '@/components/ChannelTree.vue'
import CreateChannelDialog from '@/components/channels/CreateChannelDialog.vue'
import EditChannelDialog from '@/components/channels/EditChannelDialog.vue'
import EditServerDialog from '@/components/servers/EditServerDialog.vue'
import ServerStatusInfo from '@/components/servers/ServerStatusInfo.vue'
import ACLDialog from '@/components/channels/ACLDialog.vue'
import api from '@/utils/api'

const route = useRoute()
const router = useRouter()
const store = useStore()
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
const isAdmin = computed(() => store.getters['auth/isAdmin'])

function countChannels(channels) {
  if (!channels?.length) return 0
  return channels.reduce((acc, ch) => 1 + acc + countChannels(ch.children || []), 0)
}

const totalChannelCount = computed(() => countChannels(channelTree.value))
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
const showKickDialog = ref(false)
const showMuteDialog = ref(false)
const showBanDialog = ref(false)
const kickTarget = ref(null)
const muteTarget = ref(null)
const banTarget = ref(null)
const kickReason = ref('')
const banReason = ref('')
const userActionLoading = ref(false)

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

function handleUserAction({ user, action }) {
  if (!isAdmin.value) return
  if (action === 'mute') {
    muteTarget.value = user
    showMuteDialog.value = true
  } else if (action === 'kick') {
    kickTarget.value = user
    kickReason.value = ''
    showKickDialog.value = true
  } else if (action === 'ban') {
    banTarget.value = user
    banReason.value = ''
    showBanDialog.value = true
  }
}

async function confirmMute() {
  if (!isAdmin.value) return
  if (!muteTarget.value) return
  userActionLoading.value = true
  try {
    await api.post(`/servers/${serverId.value}/users/${muteTarget.value.session_id}/mute`, { mute: !muteTarget.value.mute })
    showMuteDialog.value = false
    muteTarget.value = null
    loadUsers()
  } catch (e) {
    error.value = e.message || 'Failed to update mute'
  } finally {
    userActionLoading.value = false
  }
}

async function confirmKick() {
  if (!isAdmin.value) return
  if (!kickTarget.value) return
  userActionLoading.value = true
  try {
    await api.post(`/servers/${serverId.value}/users/${kickTarget.value.session_id}/kick`, { reason: kickReason.value || undefined })
    showKickDialog.value = false
    kickTarget.value = null
    kickReason.value = ''
    loadUsers()
  } catch (e) {
    error.value = e.message || 'Failed to kick user'
  } finally {
    userActionLoading.value = false
  }
}

async function confirmBan() {
  if (!isAdmin.value) return
  if (!banTarget.value) return
  userActionLoading.value = true
  try {
    await api.post(`/servers/${serverId.value}/users/${banTarget.value.session_id}/ban`, { reason: banReason.value || undefined })
    showBanDialog.value = false
    banTarget.value = null
    banReason.value = ''
    loadUsers()
    loadBans()
  } catch (e) {
    error.value = e.message || 'Failed to ban user'
  } finally {
    userActionLoading.value = false
  }
}
async function confirmDeleteServer() {
  if (!isAdmin.value) return
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
  if (!isAdmin.value) return
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
  if (!isAdmin.value) {
    bans.value = []
    bansLoading.value = false
    return
  }
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
  if (!isAdmin.value) return
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
  if (!isAdmin.value) return
  try {
    await api.delete(`/servers/${serverId.value}/bans/${b.id}`)
    loadBans()
  } catch (e) {
    error.value = e.message || 'Failed to delete ban'
  }
}

async function loadRegUsers() {
  if (!isAdmin.value) {
    regUsers.value = []
    regUsersLoading.value = false
    return
  }
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
  if (!isAdmin.value) return
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
  if (!isAdmin.value) return
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
    if (isAdmin.value) {
      loadBans()
      loadRegUsers()
    }
  })
})

watch(serverId, () => {
  loadServer()
  loadChannels()
  loadUsers()
  if (isAdmin.value) {
    loadBans()
    loadRegUsers()
  } else {
    bans.value = []
    regUsers.value = []
  }
})

watch(isAdmin, (next) => {
  if (next) {
    loadBans()
    loadRegUsers()
    return
  }
  bans.value = []
  regUsers.value = []
})
</script>
