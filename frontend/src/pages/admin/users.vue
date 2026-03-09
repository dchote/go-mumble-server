<template>
  <v-container>
    <StandardCard title="System Users" title-class="text-h5">
      <p class="text-body-2 text-medium-emphasis mb-4">
        These users can log in to the management UI and perform administration tasks. They are separate from
        virtual server registered users, who authenticate to Mumble voice channels on each server.
      </p>
      <v-alert v-if="error" type="error" density="compact" class="mb-4">{{ error }}</v-alert>
      <v-table v-if="users.length > 0" density="comfortable">
        <thead>
          <tr>
            <th>Username</th>
            <th>Role</th>
            <th style="width: 48px"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in users" :key="u.id">
            <td>{{ u.username }}</td>
            <td>{{ u.role }}</td>
            <td>
              <v-menu location="bottom end">
                <template #activator="{ props: menuProps }">
                  <v-btn v-bind="menuProps" icon="mdi-dots-vertical" variant="text" size="x-small" />
                </template>
                <v-list density="compact">
                  <v-tooltip
                    v-if="u.id === currentUserId"
                    location="top"
                    text="You cannot edit your own role"
                  >
                    <template #activator="{ props: tooltipProps }">
                      <v-list-item
                        v-bind="tooltipProps"
                        prepend-icon="mdi-pencil"
                        title="Edit role"
                        disabled
                      />
                    </template>
                  </v-tooltip>
                  <v-list-item
                    v-else
                    prepend-icon="mdi-pencil"
                    title="Edit role"
                    @click="openEditRole(u)"
                  />
                  <v-tooltip
                    v-if="u.id === currentUserId"
                    location="top"
                    text="You cannot delete yourself"
                  >
                    <template #activator="{ props: tooltipProps }">
                      <v-list-item
                        v-bind="tooltipProps"
                        prepend-icon="mdi-delete"
                        title="Delete"
                        disabled
                      />
                    </template>
                  </v-tooltip>
                  <v-list-item
                    v-else
                    prepend-icon="mdi-delete"
                    title="Delete"
                    @click="deleteUser(u)"
                  />
                </v-list>
              </v-menu>
            </td>
          </tr>
        </tbody>
      </v-table>
      <p v-else class="text-body-2">No users.</p>
    </StandardCard>

    <EditUserRoleDialog
      v-model="showEditRoleDialog"
      :user="editingUser"
      :loading="updatingRole"
      @save="confirmEditRole"
      @update:role="(r) => { if (editingUser.value) editingUser.value.role = r }"
      @close="editingUser = null"
    />

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
import StandardCard from '@/components/common/StandardCard.vue'
import StandardDialog from '@/components/common/StandardDialog.vue'
import EditUserRoleDialog from '@/components/admin/EditUserRoleDialog.vue'
import api from '@/utils/api'

const store = useStore()
const { mobile } = useDisplay()
const users = ref([])
const error = ref('')
const showEditRoleDialog = ref(false)
const editingUser = ref(null)
const updatingRole = ref(false)
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

function openEditRole(u) {
  if (u.id === currentUserId.value) return
  editingUser.value = { ...u }
  showEditRoleDialog.value = true
}

async function confirmEditRole() {
  if (!editingUser.value) return
  error.value = ''
  updatingRole.value = true
  try {
    await api.patch(`/users/${editingUser.value.id}`, { role: editingUser.value.role })
    const u = users.value.find((x) => x.id === editingUser.value.id)
    if (u) u.role = editingUser.value.role
    showEditRoleDialog.value = false
    editingUser.value = null
  } catch (e) {
    error.value = e.message || 'Failed to update role'
  } finally {
    updatingRole.value = false
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
