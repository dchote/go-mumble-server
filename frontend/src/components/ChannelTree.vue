<template>
  <div class="channel-tree">
    <ChannelTreeNode
      v-for="ch in channels"
      :key="ch.id"
      :channel="ch"
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
})
</script>
