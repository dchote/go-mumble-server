<template>
  <v-container>
    <v-row justify="center">
      <v-col cols="12" sm="8" md="6">
        <StandardCard title="Sign up">
          <v-form @submit.prevent="handleRegister">
              <v-alert v-if="!hasUsers" type="info" density="compact" class="mb-4">
                No administrator account exists yet. Create the first admin account below.
              </v-alert>
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
                autocomplete="new-password"
                hint="At least 8 characters"
              />
              <div class="d-flex justify-end mt-4">
                <v-btn type="submit" color="primary" variant="elevated" :loading="loading">
                  Sign up
                </v-btn>
              </div>
          </v-form>
          <p v-if="hasUsers" class="text-body-2 mt-4 text-center">
            Already have an account?
            <router-link to="/login" class="text-primary text-decoration-none font-weight-medium">Login</router-link>
          </p>
        </StandardCard>
      </v-col>
    </v-row>
  </v-container>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useStore } from 'vuex'
import { useRouter } from 'vue-router'
import StandardCard from '@/components/common/StandardCard.vue'

const store = useStore()
const router = useRouter()
const hasUsers = computed(() => store.getters['auth/hasUsers'])
const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function handleRegister() {
  error.value = ''
  if (password.value.length < 8) {
    error.value = 'Password must be at least 8 characters'
    return
  }
  loading.value = true
  try {
    await store.dispatch('auth/register', { username: username.value, password: password.value })
    router.push('/')
  } catch (e) {
    error.value = e.message || 'Registration failed'
  } finally {
    loading.value = false
  }
}
</script>
