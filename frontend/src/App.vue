<template>
  <v-app>
    <transition name="layout-fade" mode="out-in">
      <AuthenticatedLayout v-if="isAuthenticated" key="authenticated">
        <router-view />
      </AuthenticatedLayout>
      <DefaultLayout v-else key="default">
        <router-view />
      </DefaultLayout>
    </transition>
  </v-app>
</template>

<script setup>
import { computed } from 'vue'
import { useStore } from 'vuex'
import AuthenticatedLayout from '@/layouts/AuthenticatedLayout.vue'
import DefaultLayout from '@/layouts/DefaultLayout.vue'

const store = useStore()
const isAuthenticated = computed(() => store.getters['auth/isAuthenticated'])
</script>

<style>
.layout-fade-enter-active,
.layout-fade-leave-active {
  transition: opacity 0.3s ease;
}
.layout-fade-enter-from,
.layout-fade-leave-to {
  opacity: 0;
}
</style>
