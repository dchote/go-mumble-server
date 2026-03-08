<template>
  <div class="channel-node mb-1">
    <v-list-item
      :prepend-icon="channel.children?.length ? 'mdi-folder' : 'mdi-pound'"
      :title="channel.name"
      :subtitle="channel.description ? channel.description : (channel.max_users ? `Max ${channel.max_users} users` : undefined)"
      density="compact"
    />
    <div v-if="channel.children?.length" class="ml-6">
      <ChannelTreeNode
        v-for="child in channel.children"
        :key="child.id"
        :channel="child"
      />
    </div>
  </div>
</template>

<script setup>
defineProps({
  channel: {
    type: Object,
    required: true,
  },
})
</script>
