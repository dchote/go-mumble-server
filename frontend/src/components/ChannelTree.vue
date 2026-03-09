<template>
  <div class="channel-tree">
    <ChannelTreeNode
      v-for="ch in channels"
      :key="ch.id"
      :channel="ch"
      :server-id="serverId"
      :users="users"
      @create-sub="(parent) => $emit('create-sub', parent)"
      @edit="(ch) => $emit('edit', ch)"
      @acl="(ch) => $emit('acl', ch)"
      @delete="(ch) => $emit('delete', ch)"
    />
  </div>
</template>

<script setup>
import ChannelTreeNode from './ChannelTreeNode.vue'

defineProps({
  channels: {
    type: Array,
    default: () => [],
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
</script>
