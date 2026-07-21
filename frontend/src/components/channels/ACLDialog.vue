<template>
  <StandardDialog
    v-model="model"
    title="ACL & Groups"
    max-width="900"
    :persistent="true"
    @close="reset"
  >
    <v-alert v-if="error" type="error" density="compact" class="mb-4">{{ error }}</v-alert>
    <p class="text-body-2 text-medium-emphasis mb-4">
      Access control for {{ channel?.name || 'channel' }}. Changes apply to this channel and optionally sub-channels.
    </p>

    <v-tabs v-model="activeTab" density="compact" class="mb-4">
      <v-tab value="groups">Groups</v-tab>
      <v-tab value="acls">ACL entries</v-tab>
    </v-tabs>

    <v-window v-model="activeTab" class="mb-4">
      <v-window-item value="groups">
        <div class="d-flex align-center mb-3">
          <v-btn size="small" color="primary" variant="tonal" @click="addGroup">Add group</v-btn>
        </div>
        <v-table v-if="editedGroups.length" density="compact">
          <thead>
            <tr>
              <th>Name</th>
              <th>Inherit</th>
              <th>Inheritable</th>
              <th>Add user IDs</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(g, i) in editedGroups" :key="i">
              <td>
                <v-text-field v-model="g.name" density="compact" hide-details variant="outlined" autocomplete="off" />
              </td>
              <td>
                <v-checkbox v-model="g.inherit" hide-details density="compact" />
              </td>
              <td>
                <v-checkbox v-model="g.inheritable" hide-details density="compact" />
              </td>
              <td>
                <v-text-field
                  :model-value="g.add_user_ids?.join(', ') || ''"
                  @update:model-value="g.add_user_ids = parseIds($event)"
                  density="compact"
                  hide-details
                  variant="outlined"
                  placeholder="1, 2, 3"
                  autocomplete="off"
                />
              </td>
              <td>
                <v-btn icon size="small" variant="text" color="error" @click="removeGroup(i)">
                  <v-icon>mdi-delete</v-icon>
                </v-btn>
              </td>
            </tr>
          </tbody>
        </v-table>
        <p v-else class="text-body-2 text-medium-emphasis">No groups. Add one to assign users to roles.</p>
      </v-window-item>

      <v-window-item value="acls">
        <div class="d-flex align-center mb-3">
          <v-btn size="small" color="primary" variant="tonal" @click="addACL">Add ACL rule</v-btn>
        </div>
        <v-table v-if="editedACLs.length" density="compact">
          <thead>
            <tr>
              <th>User/Group/Token</th>
              <th>Apply here</th>
              <th>Apply subs</th>
              <th>Eval here (~)</th>
              <th>Invert (!)</th>
              <th>Grant</th>
              <th>Deny</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(a, i) in editedACLs" :key="i">
              <td>
                <v-select
                  v-model="a.selectorType"
                  :items="['group', 'user', 'token']"
                  density="compact"
                  hide-details
                  variant="outlined"
                  class="mb-1"
                  @update:model-value="clearSelector(a)"
                />
                <v-text-field
                  v-if="a.selectorType === 'group'"
                  v-model="a.group_name"
                  density="compact"
                  hide-details
                  variant="outlined"
                  placeholder="admin, all, auth, in, out, sub"
                  autocomplete="off"
                />
                <v-text-field
                  v-else-if="a.selectorType === 'user'"
                  v-model.number="a.user_id"
                  type="number"
                  density="compact"
                  hide-details
                  variant="outlined"
                  placeholder="User ID"
                  autocomplete="off"
                />
                <v-text-field
                  v-else
                  v-model="a.access_token"
                  density="compact"
                  hide-details
                  variant="outlined"
                  placeholder="Token"
                  autocomplete="off"
                />
              </td>
              <td>
                <v-checkbox v-model="a.apply_here" hide-details density="compact" />
              </td>
              <td>
                <v-checkbox v-model="a.apply_subs" hide-details density="compact" />
              </td>
              <td>
                <v-checkbox v-model="a.eval_here" hide-details density="compact" />
              </td>
              <td>
                <v-checkbox v-model="a.invert" hide-details density="compact" />
              </td>
              <td>
                <v-btn
                  size="small"
                  variant="tonal"
                  color="success"
                  density="compact"
                  @click="openPermEditor(i, 'grant')"
                >
                  {{ permNames(a.grant).join(', ') || '—' }}
                </v-btn>
              </td>
              <td>
                <v-btn
                  size="small"
                  variant="tonal"
                  color="error"
                  density="compact"
                  @click="openPermEditor(i, 'deny')"
                >
                  {{ permNames(a.deny).join(', ') || '—' }}
                </v-btn>
              </td>
              <td>
                <v-btn icon size="small" variant="text" color="error" @click="removeACL(i)">
                  <v-icon>mdi-delete</v-icon>
                </v-btn>
              </td>
            </tr>
          </tbody>
        </v-table>
        <p v-else class="text-body-2 text-medium-emphasis">No ACL rules. Inheritance from parent applies.</p>
      </v-window-item>
    </v-window>

    <v-dialog v-model="permDialog" max-width="500" persistent>
      <v-card>
        <v-card-title>Edit {{ permField === 'grant' ? 'Grant' : 'Deny' }} permissions</v-card-title>
        <v-card-text>
          <v-chip-group>
            <v-chip
              v-for="p in PERMISSIONS"
              :key="p.value"
              :color="permField === 'grant' ? 'success' : 'error'"
              :variant="hasPerm(permEditValue, p.value) ? 'flat' : 'outlined'"
              size="small"
              @click="togglePerm(p.value)"
            >
              {{ p.name }}
            </v-chip>
          </v-chip-group>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="permDialog = false">Done</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <template #actions>
      <v-spacer />
      <v-btn variant="text" @click="model = false">Cancel</v-btn>
      <v-btn color="primary" :loading="saving" :disabled="!dirty" @click="save">Save</v-btn>
    </template>
  </StandardDialog>
</template>

<script setup>
import { ref, watch, computed } from 'vue'
import StandardDialog from '@/components/common/StandardDialog.vue'
import api from '@/utils/api'
import { PERMISSIONS, hasPermission, setPermission } from '@/utils/permissions'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  serverId: { type: [String, Number], required: true },
  channel: { type: Object, default: null },
})
const emit = defineEmits(['update:modelValue'])

const model = ref(props.modelValue)
watch(() => props.modelValue, (v) => { model.value = v })
watch(model, (v) => emit('update:modelValue', v))

const error = ref('')
const saving = ref(false)
const activeTab = ref('groups')
const editedGroups = ref([])
const editedACLs = ref([])
const originalGroups = ref([])
const originalACLs = ref([])
const permDialog = ref(false)
const permEditIndex = ref(-1)
const permField = ref('grant')
const permEditValue = ref(0)

const dirty = computed(() => {
  return JSON.stringify(editedGroups.value) !== JSON.stringify(originalGroups.value) ||
    JSON.stringify(editedACLs.value.map(normalizeACL)) !== JSON.stringify(originalACLs.value.map(normalizeACL))
})

function permNames(value) {
  const v = value || 0
  if (v & 0x1) return ['Write']
  return PERMISSIONS.filter((p) => p.value !== 0x1 && (v & p.value) === p.value).map((p) => p.name)
}

function normalizeACL(a) {
  const { selectorType: _selectorType, ...rest } = a
  return rest
}

function parseIds(s) {
  if (!s || typeof s !== 'string') return []
  return s.split(/[,\s]+/).map((x) => parseInt(x.trim(), 10)).filter((n) => !isNaN(n) && n > 0)
}

function clearSelector(a) {
  a.user_id = null
  a.group_name = ''
  a.access_token = ''
}

function addGroup() {
  editedGroups.value.push({
    name: '',
    inherit: true,
    inheritable: true,
    add_user_ids: [],
    remove_user_ids: [],
  })
}

function removeGroup(i) {
  editedGroups.value.splice(i, 1)
}

function addACL() {
  editedACLs.value.push({
    selectorType: 'group',
    priority: editedACLs.value.length,
    apply_here: true,
    apply_subs: true,
    eval_here: false,
    invert: false,
    user_id: null,
    group_name: 'all',
    access_token: '',
    grant: 0,
    deny: 0,
  })
}

function removeACL(i) {
  editedACLs.value.splice(i, 1)
}

function openPermEditor(index, field) {
  permEditIndex.value = index
  permField.value = field
  permEditValue.value = editedACLs.value[index]?.[field] || 0
  permDialog.value = true
}

function hasPerm(mask, val) {
  return hasPermission(mask, val)
}

function togglePerm(val) {
  permEditValue.value = setPermission(permEditValue.value, val, permField.value === 'grant')
  if (permEditIndex.value >= 0 && editedACLs.value[permEditIndex.value]) {
    editedACLs.value[permEditIndex.value][permField.value] = permEditValue.value
  }
}

watch([() => props.modelValue, () => props.channel, () => props.serverId], async ([open, ch, sid]) => {
  if (open && ch && sid) {
    error.value = ''
    try {
      const data = await api.get(`/servers/${sid}/channels/${ch.id}/acl`)
      editedGroups.value = (data.groups || []).map((g) => ({
        name: g.name,
        inherit: g.inherit ?? true,
        inheritable: g.inheritable ?? true,
        add_user_ids: g.add_user_ids || [],
        remove_user_ids: g.remove_user_ids || [],
      }))
      editedACLs.value = (data.acls || []).map((a) => ({
        selectorType: a.user_id != null ? 'user' : (a.access_token ? 'token' : 'group'),
        priority: a.priority ?? 0,
        apply_here: a.apply_here ?? true,
        apply_subs: a.apply_subs ?? true,
        eval_here: a.eval_here ?? false,
        invert: a.invert ?? false,
        user_id: a.user_id ?? null,
        group_name: a.group_name || '',
        access_token: a.access_token || '',
        grant: a.grant || 0,
        deny: a.deny || 0,
      }))
      originalGroups.value = JSON.parse(JSON.stringify(editedGroups.value))
      originalACLs.value = editedACLs.value.map(normalizeACL)
    } catch (e) {
      error.value = e.message || 'Failed to load ACL'
      editedGroups.value = []
      editedACLs.value = []
      originalGroups.value = []
      originalACLs.value = []
    }
  }
}, { immediate: true })

async function save() {
  if (!props.channel || !props.serverId) return
  saving.value = true
  error.value = ''
  try {
    const groups = editedGroups.value.filter((g) => g.name.trim()).map((g) => ({
      name: g.name.trim(),
      inherit: g.inherit,
      inheritable: g.inheritable,
      add_user_ids: g.add_user_ids || [],
      remove_user_ids: g.remove_user_ids || [],
    }))
    const acls = editedACLs.value.map((a, i) => {
      const out = {
        priority: i,
        apply_here: a.apply_here,
        apply_subs: a.apply_subs,
        eval_here: a.eval_here,
        invert: a.invert,
        grant: a.grant || 0,
        deny: a.deny || 0,
      }
      if (a.selectorType === 'user' && a.user_id != null) {
        out.user_id = a.user_id
        out.group_name = ''
        out.access_token = ''
      } else if (a.selectorType === 'token' && a.access_token) {
        out.access_token = a.access_token
        out.user_id = null
        out.group_name = ''
      } else {
        out.group_name = a.group_name || 'all'
        out.user_id = null
        out.access_token = ''
      }
      return out
    })
    await api.put(`/servers/${props.serverId}/channels/${props.channel.id}/acl`, { groups, acls })
    originalGroups.value = JSON.parse(JSON.stringify(editedGroups.value))
    originalACLs.value = editedACLs.value.map(normalizeACL)
    model.value = false
  } catch (e) {
    error.value = e.message || 'Failed to save ACL'
  } finally {
    saving.value = false
  }
}

function reset() {
  error.value = ''
  activeTab.value = 'groups'
}
</script>
