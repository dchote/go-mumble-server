<template>
  <div class="channel-node mb-1">
    <v-list-item
      :prepend-icon="channel.children?.length ? 'mdi-folder' : 'mdi-pound'"
      :title="channel.name"
      :subtitle="channel.description ? channel.description : (channel.max_users ? `Max ${channel.max_users} users` : undefined)"
      density="compact"
      @click="onRowClick"
    >
      <template v-if="serverId" #append>
        <div class="channel-menu" @click.stop>
          <v-menu location="bottom end">
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
        :title="userStatusTooltip(u)"
      >
        <template #prepend>
          <v-icon size="x-small" :color="u.self_mute || u.mute ? 'warning' : (u.self_deaf || u.deaf ? 'default' : 'success')">
            {{ u.self_deaf || u.deaf ? 'mdi-ear-off' : (u.self_mute || u.mute ? 'mdi-microphone-off' : 'mdi-account-voice') }}
          </v-icon>
        </template>
        <v-list-item-title class="text-body-2">{{ u.name || u.username || 'Unknown' }}</v-list-item-title>
        <template #append>
          <v-chip
            v-if="u.is_admin || u.isAdmin"
            size="x-small"
            color="primary"
            variant="tonal"
            density="compact"
            class="mr-2"
          >
            Admin
          </v-chip>
          <span class="text-caption text-medium-emphasis">Session #{{ u.session_id }}</span>
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
        @create-sub="$emit('create-sub', $event)"
        @edit="$emit('edit', $event)"
        @acl="$emit('acl', $event)"
        @delete="$emit('delete', $event)"
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
})
defineEmits(['create-sub', 'edit', 'acl', 'delete'])

const channelUsers = computed(() => {
  if (!props.users?.length) return []
  const cid = typeof props.channel.id === 'string' ? parseInt(props.channel.id, 10) : props.channel.id
  return props.users.filter((u) => (u.channel_id ?? u.channelId) === cid)
})

function userStatusTooltip(u) {
  const parts = [`Session ${u.session_id}`]
  if (u.is_admin || u.isAdmin) parts.push('Admin')
  if (u.self_mute || u.mute) parts.push('Muted')
  else if (u.self_deaf || u.deaf) parts.push('Deafened')
  else parts.push('Speaking')
  return parts.join(' · ')
}

const router = useRouter()
function onRowClick() {
  if (props.serverId) {
    router.push(`/servers/${props.serverId}/channels/${props.channel.id}`)
  }
}
</script>
