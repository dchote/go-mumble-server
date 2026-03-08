<template>
  <v-container>
    <BrandCard title="Users" title-class="text-h5" content-class="pa-3 pa-sm-6">
      <v-alert v-if="error" type="error" density="compact" class="mb-4">{{ error }}</v-alert>
      <v-table v-if="users.length > 0">
        <thead>
          <tr>
            <th>Username</th>
            <th>Role</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in users" :key="u.id">
            <td>{{ u.username }}</td>
            <td>
              <v-select
                v-if="u.id !== currentUserId"
                :model-value="u.role"
                :items="['admin', 'user']"
                variant="outlined"
                density="compact"
                hide-details="auto"
                class="mr-2"
                style="max-width: 120px"
                @update:model-value="(r) => updateRole(u.id, r)"
              />
              <span v-else>{{ u.role }}</span>
            </td>
            <td>
              <v-btn
                v-if="u.id !== currentUserId"
                size="small"
                color="error"
                variant="text"
                @click="deleteUser(u)"
              >
                Delete
              </v-btn>
            </td>
          </tr>
        </tbody>
      </v-table>
      <p v-else class="text-body-2">No users.</p>
    </BrandCard>

    <StandardDialog
      v-model="showDeleteDialog"
      title="Delete user?"
      max-width="400"
      :fullscreen="mobile"
      @close="userToDelete = null"
    >
      <p>Are you sure you want to delete {{ userToDelete?.username }}?</p>
      <template #actions>
        <v-spacer />
        <v-btn variant="text" class="mr-2" @click="showDeleteDialog = false">Cancel</v-btn>
        <v-btn color="error" variant="elevated" :loading="deleting" @click="confirmDelete">
          Delete
        </v-btn>
      </template>
    </StandardDialog>
  </v-container>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useStore } from 'vuex'
import { useDisplay } from 'vuetify'
import BrandCard from '@/components/common/BrandCard.vue'
import StandardDialog from '@/components/common/StandardDialog.vue'
import api from '@/utils/api'

const store = useStore()
const { mobile } = useDisplay()
const users = ref([])
const error = ref('')
const showDeleteDialog = ref(false)
const userToDelete = ref(null)
const deleting = ref(false)

const currentUserId = computed(() => store.getters['auth/user']?.id)

onMounted(async () => {
  try {
    users.value = await api.get('/users')
  } catch (e) {
    error.value = e.message || 'Failed to load users'
  }
})

async function updateRole(id, role) {
  error.value = ''
  try {
    await api.patch(`/users/${id}`, { role })
    const u = users.value.find((x) => x.id === id)
    if (u) u.role = role
  } catch (e) {
    error.value = e.message || 'Failed to update role'
  }
}

function deleteUser(u) {
  userToDelete.value = u
  showDeleteDialog.value = true
}

async function confirmDelete() {
  if (!userToDelete.value) return
  error.value = ''
  deleting.value = true
  try {
    await api.delete(`/users/${userToDelete.value.id}`)
    users.value = users.value.filter((x) => x.id !== userToDelete.value.id)
    showDeleteDialog.value = false
    userToDelete.value = null
  } catch (e) {
    error.value = e.message || 'Failed to delete user'
  } finally {
    deleting.value = false
  }
}
</script>
