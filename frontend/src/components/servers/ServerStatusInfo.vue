<template>
  <v-row>
    <v-col
      v-if="showVirtualServerCount && virtualServerCount != null"
      cols="12"
      sm="6"
      md="4"
      lg="3"
    >
      <div class="metric-card">
        <div class="metric-icon metric-icon--purple">
          <v-icon icon="mdi-server" size="24" color="white" />
        </div>
        <div class="metric-content">
          <div class="metric-value">{{ formatValue(virtualServerCount) }}</div>
          <div class="metric-label">Virtual servers</div>
        </div>
      </div>
    </v-col>
    <v-col v-if="channelCount != null" cols="12" sm="6" md="4" lg="3">
      <div class="metric-card">
        <div class="metric-icon metric-icon--blue">
          <v-icon icon="mdi-pound" size="24" color="white" />
        </div>
        <div class="metric-content">
          <div class="metric-value">{{ formatValue(channelCount) }}</div>
          <div class="metric-label">Channels</div>
        </div>
      </div>
    </v-col>
    <v-col
      v-if="connectedUsers != null || maxUsers != null"
      cols="12"
      sm="6"
      md="4"
      lg="3"
    >
      <div class="metric-card">
        <div class="metric-icon metric-icon--pink">
          <v-icon icon="mdi-account-voice" size="24" color="white" />
        </div>
        <div class="metric-content">
          <div class="metric-value">{{ usersDisplay }}</div>
          <div class="metric-label">Users</div>
        </div>
      </div>
    </v-col>
  </v-row>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  virtualServerCount: {
    type: [Number, String],
    default: undefined,
  },
  channelCount: {
    type: [Number, String],
    default: undefined,
  },
  maxUsers: {
    type: [Number, String],
    default: undefined,
  },
  connectedUsers: {
    type: [Number, String],
    default: undefined,
  },
  showVirtualServerCount: {
    type: Boolean,
    default: false,
  },
})

function formatValue(v) {
  if (v == null || v === '') return '-'
  if (typeof v === 'number') return v.toLocaleString()
  return String(v)
}

const usersDisplay = computed(() => {
  const conn = props.connectedUsers ?? 0
  const max = props.maxUsers
  if (max != null && max !== '') {
    return `${formatValue(conn)}/${formatValue(max)}`
  }
  return formatValue(conn)
})
</script>

<style scoped>
.metric-card {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 1rem 1.25rem;
  height: 100%;
  background: rgb(var(--v-theme-surface));
  border-radius: 8px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
  min-width: 0;
}

.metric-icon {
  flex-shrink: 0;
  width: 48px;
  height: 48px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.metric-icon--purple {
  background: #8468d6;
}

.metric-icon--blue {
  background: #668ee0;
}

.metric-icon--pink {
  background: #e86b97;
}

.metric-content {
  min-width: 0;
}

.metric-value {
  font-size: 1.5rem;
  font-weight: 700;
  line-height: 1.2;
  color: rgb(var(--v-theme-on-surface));
}

.metric-label {
  font-size: 0.875rem;
  font-weight: 400;
  color: rgba(var(--v-theme-on-surface), 0.7);
  margin-top: 2px;
}
</style>
