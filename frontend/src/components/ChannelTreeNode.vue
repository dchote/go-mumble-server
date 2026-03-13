<template>
  <div class="channel-node mb-1">
    <v-list-item
      :prepend-icon="channel.children?.length ? 'mdi-folder' : 'mdi-pound'"
      :title="channel.name"
      :subtitle="channel.description ? channel.description : (channel.max_users ? `Max ${channel.max_users} users` : undefined)"
      density="compact"
      @click="onRowClick"
    >
      <template #append>
        <div class="d-flex align-center" @click.stop>
          <v-chip
            v-if="channel.crypto_mode"
            size="x-small"
            variant="tonal"
            density="compact"
            :color="cryptoModeColor"
            class="mr-2"
          >
            {{ channel.crypto_mode }}
          </v-chip>
          <v-chip v-if="channelUsers.length > 0" size="x-small" variant="tonal" density="compact" class="mr-2">
            {{ channelUsers.length }}
          </v-chip>
          <v-menu v-if="serverId && canManageUsers" location="bottom end">
            <template #activator="{ props: menuProps }">
              <v-btn icon="mdi-dots-vertical" variant="text" size="x-small" v-bind="menuProps" />
            </template>
            <v-list density="compact">
              <v-list-item prepend-icon="mdi-plus" title="Create subchannel" @click="$emit('create-sub', channel)" />
              <v-list-item prepend-icon="mdi-pencil" title="Edit" @click="$emit('edit', channel)" />
              <v-list-item prepend-icon="mdi-shield-account" title="ACL" @click="$emit('acl', channel)" />
              <v-list-item
                v-if="channel.id !== 0"
                prepend-icon="mdi-delete"
                title="Delete"
                @click="$emit('delete', channel)"
              />
            </v-list>
          </v-menu>
        </div>
      </template>
    </v-list-item>
    <div v-if="channelUsers.length" class="ml-6 mb-1">
      <v-list-item
        v-for="u in channelUsers"
        :key="u.session_id"
        density="compact"
        class="channel-user pl-4"
        :subtitle="showSensitiveUserData ? (u.address || '-') : undefined"
      >
        <template #prepend>
          <v-icon size="x-small" :color="u.self_mute || u.mute ? 'warning' : (u.self_deaf || u.deaf ? 'default' : 'success')">
            {{ u.self_deaf || u.deaf ? 'mdi-ear-off' : (u.self_mute || u.mute ? 'mdi-microphone-off' : 'mdi-account-voice') }}
          </v-icon>
        </template>
        <v-list-item-title class="text-body-2">{{ u.name || u.username || 'Unknown' }}</v-list-item-title>
        <template #append>
          <div class="d-flex align-center">
            <v-chip
              v-if="(u.crypto_mode || u.cryptoMode)"
              size="x-small"
              variant="tonal"
              density="compact"
              color="secondary"
              class="mr-1"
            >
              {{ (u.crypto_mode || u.cryptoMode) }}
            </v-chip>
            <v-chip
              v-if="u.voice_transport || u.voiceTransport"
              size="x-small"
              variant="tonal"
              density="compact"
              :color="(u.voice_transport || u.voiceTransport) === 'udp' ? 'success' : 'info'"
              class="mr-1"
            >
              {{ (u.voice_transport || u.voiceTransport).toUpperCase() }}
            </v-chip>
            <v-chip
              v-if="u.is_admin || u.isAdmin"
              size="x-small"
              color="primary"
              variant="tonal"
              density="compact"
              class="mr-1"
            >
              Admin
            </v-chip>
            <v-chip
              v-if="u.self_mute"
              size="x-small"
              variant="tonal"
              density="compact"
              color="warning"
              class="mr-1"
            >
              Self-muted
            </v-chip>
            <v-chip
              v-if="u.self_deaf"
              size="x-small"
              variant="tonal"
              density="compact"
              color="warning"
              class="mr-1"
            >
              Self-deaf
            </v-chip>
            <v-chip
              v-if="u.mute"
              size="x-small"
              variant="tonal"
              density="compact"
              color="error"
              class="mr-1"
            >
              Muted
            </v-chip>
            <v-chip
              v-if="u.deaf"
              size="x-small"
              variant="tonal"
              density="compact"
              color="error"
              class="mr-1"
            >
              Deaf
            </v-chip>
            <v-menu v-if="serverId && canManageUsers" location="bottom end" @click.stop>
              <template #activator="{ props: menuProps }">
                <v-btn icon="mdi-dots-vertical" variant="text" size="x-small" v-bind="menuProps" class="ml-1" />
              </template>
              <v-list density="compact">
                <v-list-item prepend-icon="mdi-block-helper" title="Ban" @click="onUserAction(u, 'ban')" />
                <v-list-item
                  :prepend-icon="u.mute ? 'mdi-microphone' : 'mdi-microphone-off'"
                  :title="u.mute ? 'Unmute' : 'Mute'"
                  @click="onUserAction(u, 'mute')"
                />
                <v-list-item prepend-icon="mdi-account-arrow-right" title="Kick" @click="onUserAction(u, 'kick')" />
              </v-list>
            </v-menu>
          </div>
        </template>
      </v-list-item>
    </div>
    <div v-if="channel.children?.length" class="ml-6">
      <ChannelTreeNode
        v-for="child in channel.children"
        :key="child.id"
        :channel="child"
        :server-id="serverId"
        :users="users"
        :can-manage-users="canManageUsers"
        :show-sensitive-user-data="showSensitiveUserData"
        @create-sub="$emit('create-sub', $event)"
        @edit="$emit('edit', $event)"
        @acl="$emit('acl', $event)"
        @delete="$emit('delete', $event)"
        @user-action="$emit('user-action', $event)"
      />
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRouter } from 'vue-router'

const props = defineProps({
  channel: {
    type: Object,
    required: true,
  },
  serverId: {
    type: [String, Number],
    default: '',
  },
  users: {
    type: Array,
    default: () => [],
  },
  canManageUsers: {
    type: Boolean,
    default: false,
  },
  showSensitiveUserData: {
    type: Boolean,
    default: false,
  },
})
const emit = defineEmits(['create-sub', 'edit', 'acl', 'delete', 'user-action'])

const channelUsers = computed(() => {
  if (!props.users?.length) return []
  const cid = typeof props.channel.id === 'string' ? parseInt(props.channel.id, 10) : props.channel.id
  return props.users.filter((u) => (u.channel_id ?? u.channelId) === cid)
})

const cryptoModeColor = computed(() => {
  const colors = { legacy: 'secondary', lite: 'info', secure: 'success', mixed: 'warning' }
  return colors[props.channel.crypto_mode] || 'secondary'
})

function onUserAction(u, action) {
  emit('user-action', { user: u, action })
}

const router = useRouter()
function onRowClick() {
  if (props.serverId) {
    router.push(`/servers/${props.serverId}/channels/${props.channel.id}`)
  }
}
</script>
