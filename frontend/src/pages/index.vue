<template>
  <v-container>
    <v-row>
      <v-col cols="12">
        <BrandCard title="Dashboard">
          <p class="text-subtitle-2 text-medium-emphasis mb-2">go-mumble-server Management</p>
          <p>Welcome, {{ user?.username }}.</p>
            <p class="mt-2">
              <v-chip color="success" size="small">Connected</v-chip>
              REST API at <code>/api/v1</code>
            </p>
            <p class="mt-2">
              <v-btn variant="text" href="/docs" target="_blank">
                <v-icon left>mdi-book-open-variant</v-icon>
                Swagger API docs
              </v-btn>
            </p>
        </BrandCard>
      </v-col>
    </v-row>
  </v-container>
</template>

<script setup>
import { computed } from 'vue'
import { useStore } from 'vuex'
import BrandCard from '@/components/common/BrandCard.vue'

const store = useStore()
const user = computed(() => store.getters['auth/user'])
</script>
