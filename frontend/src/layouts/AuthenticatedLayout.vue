<template>
  <div>
    <v-app-bar app color="primary" density="comfortable">
      <v-app-bar-title class="font-weight-bold">
        <router-link to="/" class="text-white text-decoration-none">go-mumble-server</router-link>
      </v-app-bar-title>
      <v-spacer />
      <v-chip size="small" variant="flat" class="mr-2">{{ user?.username }}</v-chip>
      <v-btn icon variant="text" @click="logout" title="Logout">
        <v-icon>mdi-logout</v-icon>
      </v-btn>
    </v-app-bar>
    <v-navigation-drawer v-model="drawer" :rail="rail" permanent>
      <v-list nav density="comfortable">
        <v-list-item prepend-icon="mdi-view-dashboard" title="Dashboard" to="/" />
        <v-list-item v-if="isAdmin" prepend-icon="mdi-account-multiple" title="Users" to="/admin/users" />
      </v-list>
    </v-navigation-drawer>
    <v-main>
      <div class="pa-4">
        <slot />
      </div>
    </v-main>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useStore } from 'vuex'
import { useRouter } from 'vue-router'

const store = useStore()
const router = useRouter()
const drawer = ref(true)
const rail = ref(false)

const user = computed(() => store.getters['auth/user'])
const isAdmin = computed(() => store.getters['auth/isAdmin'])

async function logout() {
  await store.dispatch('auth/logout')
  router.push('/login')
}
</script>
