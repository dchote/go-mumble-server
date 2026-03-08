<template>
  <div class="default-layout">
    <v-app-bar app color="primary" density="comfortable">
      <v-app-bar-title class="font-weight-bold">
        <router-link to="/" class="text-white text-decoration-none">go-mumble-server</router-link>
      </v-app-bar-title>
      <v-spacer />
      <v-btn
        v-if="!isLoginPage && !isRegisterPage"
        size="small"
        variant="elevated"
        :to="hasUsers ? '/login' : '/register'"
      >
        {{ hasUsers ? 'Login' : 'Sign up' }}
      </v-btn>
    </v-app-bar>
    <v-main>
      <div class="pa-4">
        <slot />
      </div>
    </v-main>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useStore } from 'vuex'

const route = useRoute()
const store = useStore()

const isLoginPage = computed(() => route.path === '/login')
const isRegisterPage = computed(() => route.path === '/register')
const hasUsers = computed(() => store.getters['auth/hasUsers'])
</script>
