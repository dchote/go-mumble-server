<template>
  <v-container>
    <v-row justify="center">
      <v-col cols="12" sm="8" md="6">
        <BrandCard title="Login" content-class="pa-3 pa-sm-6">
          <v-form @submit.prevent="handleLogin">
              <v-alert v-if="error" type="error" density="compact" class="mb-4">
                {{ error }}
              </v-alert>
              <v-text-field
                v-model="username"
                label="Username"
                variant="outlined"
                density="comfortable"
                hide-details="auto"
                class="mb-4"
                autocomplete="username"
              />
              <v-text-field
                v-model="password"
                label="Password"
                type="password"
                variant="outlined"
                density="comfortable"
                hide-details="auto"
                class="mb-4"
                autocomplete="current-password"
              />
              <div class="d-flex justify-end mt-4">
                <v-btn type="submit" color="primary" variant="elevated" :loading="loading">
                  Login
                </v-btn>
              </div>
          </v-form>
          <p class="text-body-2 mt-4 text-center">
            Don't have an account?
            <router-link to="/register" class="text-primary text-decoration-none font-weight-medium">Sign up</router-link>
          </p>
        </BrandCard>
      </v-col>
    </v-row>
  </v-container>
</template>

<script setup>
import { ref } from 'vue'
import { useStore } from 'vuex'
import { useRouter } from 'vue-router'
import BrandCard from '@/components/common/BrandCard.vue'

const store = useStore()
const router = useRouter()
const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function handleLogin() {
  error.value = ''
  loading.value = true
  try {
    await store.dispatch('auth/login', { username: username.value, password: password.value })
    router.push('/')
  } catch (e) {
    error.value = e.message || 'Login failed'
  } finally {
    loading.value = false
  }
}
</script>
