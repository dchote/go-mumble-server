<template>
  <v-container>
    <BrandCard title-class="text-h5" content-class="pa-3 pa-sm-6">
      <template #header>
        <BackButton :fallback="`/servers/${serverId}`" class="mr-2" />
        <span class="text-h5 header-truncate">{{ channel?.name || `Channel ${channelId}` }}</span>
        <v-spacer />
        <v-btn variant="tonal" color="primary" size="small" class="mr-2" @click="openACL">
          ACL & Groups
        </v-btn>
        <v-btn variant="elevated" color="primary" size="small" @click="openEdit">Edit</v-btn>
      </template>
      <v-alert v-if="error" type="error" density="compact" class="mb-4">{{ error }}</v-alert>

      <v-sheet v-if="loading">
        <v-progress-linear indeterminate color="primary" />
      </v-sheet>
      <template v-else-if="channel">
        <div class="mb-4">
          <p class="text-body-2 text-medium-emphasis mb-1">
            <strong>Parent:</strong> {{ channel.parent_id != null ? channel.parent_id : 'Root' }}
          </p>
          <p v-if="channel.description" class="text-body-2 mb-0">{{ channel.description }}</p>
          <p v-else-if="channel.max_users" class="text-body-2 mb-0">Max users: {{ channel.max_users }}</p>
        </div>

        <div v-if="children.length > 0" class="section-header d-flex align-center mb-2">
          <h3 class="text-subtitle-1 font-weight-bold mb-0">Subchannels</h3>
          <v-spacer />
          <v-btn size="small" variant="elevated" color="primary" @click="openCreate">
            <v-icon start size="small">mdi-plus</v-icon>
            Add Channel
          </v-btn>
        </div>
        <div v-if="children.length > 0" class="channels-list">
          <router-link
            v-for="ch in children"
            :key="ch.id"
            :to="`/servers/${serverId}/channels/${ch.id}`"
            class="channel-list-item"
          >
            <v-icon size="small" color="grey" class="mr-2">mdi-pound</v-icon>
            <span class="text-body-1">{{ ch.name }}</span>
          </router-link>
        </div>
        <p v-else class="text-body-2 text-medium-emphasis">No subchannels. Add one or use the server page to manage the full tree.</p>
      </template>

      <EditChannelDialog v-model="showEditDialog" :server-id="serverId" :channel="channel" @updated="loadChannel" />
      <CreateChannelDialog
        v-model="showCreateDialog"
        :server-id="serverId"
        :channels="channelTree"
        :parent-id="channel?.id ?? 0"
        @created="loadChannel"
      />
      <ACLDialog v-model="showACLDialog" :server-id="serverId" :channel="channel" />
    </BrandCard>
  </v-container>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import BrandCard from '@/components/common/BrandCard.vue'
import BackButton from '@/components/common/BackButton.vue'
import EditChannelDialog from '@/components/channels/EditChannelDialog.vue'
import CreateChannelDialog from '@/components/channels/CreateChannelDialog.vue'
import ACLDialog from '@/components/channels/ACLDialog.vue'
import api from '@/utils/api'

const route = useRoute()
const serverId = computed(() => route.params.id)
const channelId = computed(() => route.params.channelId)
const channel = ref(null)
const channelTree = ref([])
const error = ref('')
const loading = ref(true)
const showEditDialog = ref(false)
const showCreateDialog = ref(false)
const showACLDialog = ref(false)

const children = computed(() => {
  const ch = channel.value
  if (!ch?.children) return []
  return ch.children
})

function openEdit() {
  showEditDialog.value = true
}

function openCreate() {
  showCreateDialog.value = true
}

function openACL() {
  showACLDialog.value = true
}

function findChannel(tree, id) {
  const idNum = Number(id)
  for (const c of tree || []) {
    if (c.id === idNum) return c
    const found = findChannel(c.children, id)
    if (found) return found
  }
  return null
}

async function loadChannel() {
  if (!serverId.value || !channelId.value) return
  loading.value = true
  error.value = ''
  try {
    const tree = await api.get(`/servers/${serverId.value}/channels`)
    channelTree.value = tree
    channel.value = findChannel(tree, channelId.value)
  } catch (e) {
    error.value = e.message || 'Failed to load channel'
  } finally {
    loading.value = false
  }
}

onMounted(loadChannel)
watch([serverId, channelId], loadChannel)
</script>

<style scoped>
.channel-list-item {
  display: flex;
  align-items: center;
  padding: 12px 0;
  text-decoration: none;
  color: inherit;
  border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
  transition: background-color 0.2s;
}

.channel-list-item:last-child {
  border-bottom: none;
}

.channel-list-item:hover {
  background-color: rgba(var(--v-theme-on-surface), 0.04);
}
</style>
