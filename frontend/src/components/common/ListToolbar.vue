<template>
  <div class="list-toolbar">
    <slot />
  </div>
</template>

<script setup>
</script>

<style scoped>
.list-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  padding: 0;
  margin: 0;
}

.list-toolbar > * {
  margin-right: 12px;
  margin-bottom: 4px;
}

.list-toolbar > *:last-child {
  margin-right: 0;
}

.list-toolbar :deep(.v-row) {
  flex: 1 1 100%;
  margin: 0;
}

.list-toolbar :deep(.v-col) {
  padding-top: 0;
  padding-bottom: 0;
}
</style>
