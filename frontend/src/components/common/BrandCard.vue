<template>
  <v-card
    :class="['brand-page-card', cardClass]"
    :elevation="elevation"
    v-bind="$attrs"
  >
    <template v-if="title || $slots.header">
      <v-card-title :class="['brand-card-title', 'd-flex', 'align-center', headerClass]">
        <slot name="header">
          <span :class="[titleClass, 'header-truncate']">{{ title }}</span>
        </slot>
      </v-card-title>
      <v-divider />
    </template>
    <div v-if="$slots.toolbar" class="brand-card-toolbar">
      <slot name="toolbar" />
    </div>
    <v-card-text :class="[$slots.toolbar ? 'brand-card-content-with-toolbar' : '', contentClass]">
      <slot />
    </v-card-text>
    <template v-if="$slots.actions">
      <v-divider />
      <v-card-actions class="card-actions">
        <slot name="actions" />
      </v-card-actions>
    </template>
  </v-card>
</template>

<script setup>
defineProps({
  title: {
    type: String,
    default: '',
  },
  cardClass: {
    type: String,
    default: '',
  },
  contentClass: {
    type: String,
    default: 'pa-4',
  },
  titleClass: {
    type: String,
    default: 'text-h6 font-weight-bold',
  },
  headerClass: {
    type: String,
    default: '',
  },
  elevation: {
    type: [Number, String],
    default: undefined,
  },
})
</script>
