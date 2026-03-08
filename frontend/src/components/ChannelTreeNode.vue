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
    <div v-if="channel.children?.length" class="ml-6">
      <ChannelTreeNode
        v-for="child in channel.children"
        :key="child.id"
        :channel="child"
        :server-id="serverId"
        @create-sub="$emit('create-sub', $event)"
        @edit="$emit('edit', $event)"
        @acl="$emit('acl', $event)"
        @delete="$emit('delete', $event)"
      />
    </div>
  </div>
</template>

<script setup>
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
})
defineEmits(['create-sub', 'edit', 'acl', 'delete'])

const router = useRouter()
function onRowClick() {
  if (props.serverId) {
    router.push(`/servers/${props.serverId}/channels/${props.channel.id}`)
  }
}
</script>
