<template>
  <StandardDialog
    v-model="model"
    title="Create channel"
    max-width="500"
    :persistent="true"
    @close="reset"
  >
    <v-form @submit.prevent="handleSubmit">
      <v-alert v-if="error" type="error" density="compact" class="mb-4">{{ error }}</v-alert>
      <v-select
        v-model="form.parent_id"
        label="Parent channel"
        :items="parentOptions"
        item-title="name"
        item-value="id"
        variant="outlined"
        density="compact"
        hide-details="auto"
        class="mb-4"
      />
      <v-text-field
        v-model="form.name"
        label="Name"
        variant="outlined"
        density="compact"
        hide-details="auto"
        autocomplete="off"
        class="mb-4"
      />
      <v-textarea
        v-model="form.description"
        label="Description"
        variant="outlined"
        density="compact"
        hide-details="auto"
        rows="2"
        autocomplete="off"
        class="mb-4"
      />
      <v-text-field
        v-model.number="form.position"
        label="Position"
        type="number"
        variant="outlined"
        density="compact"
        hide-details="auto"
        autocomplete="off"
        class="mb-4"
      />
      <v-text-field
        v-model.number="form.max_users"
        label="Max users (0 = unlimited)"
        type="number"
        variant="outlined"
        density="compact"
        hide-details="auto"
        autocomplete="off"
        class="mb-4"
      />
      <v-checkbox
        v-model="form.is_temporary"
        label="Temporary channel"
        hide-details
        density="compact"
        class="mb-4"
      />
    </v-form>
    <template #actions>
      <v-spacer />
      <v-btn variant="text" class="mr-2" @click="model = false">Cancel</v-btn>
      <v-btn color="primary" variant="elevated" :loading="loading" @click="handleSubmit">
        Create
      </v-btn>
    </template>
  </StandardDialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import StandardDialog from '@/components/common/StandardDialog.vue'
import api from '@/utils/api'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  serverId: { type: [String, Number], required: true },
  channels: { type: Array, default: () => [] },
  parentId: { type: Number, default: 0 },
})
const emit = defineEmits(['update:modelValue', 'created'])

const model = ref(props.modelValue)
watch(() => props.modelValue, (v) => { model.value = v })
watch(model, (v) => emit('update:modelValue', v))

const parentOptions = computed(() => {
  const flat = []
  const add = (chs, prefix = '') => {
    for (const c of chs || []) {
      flat.push({ id: c.id, name: prefix + (c.name || 'Unnamed') })
      if (c.children?.length) add(c.children, prefix + '  ')
    }
  }
  add(props.channels)
  return flat
})

const form = ref({
  parent_id: props.parentId ?? 0,
  name: '',
  description: '',
  position: 0,
  max_users: 0,
  is_temporary: false,
})
const loading = ref(false)
const error = ref('')

watch([() => props.modelValue, () => props.parentId, () => props.channels], ([open, pid, chs]) => {
  if (open) {
    const rootId = chs?.[0]?.id ?? 0
    form.value = {
      parent_id: (pid === 0 || pid == null) ? rootId : pid,
      name: '',
      description: '',
      position: 0,
      max_users: 0,
      is_temporary: false,
    }
    error.value = ''
  }
})

function reset() {
  error.value = ''
  loading.value = false
}

async function handleSubmit() {
  if (!form.value.name?.trim()) {
    error.value = 'Name is required'
    return
  }
  error.value = ''
  loading.value = true
  try {
    const ch = await api.post(`/servers/${props.serverId}/channels`, {
      parent_id: form.value.parent_id,
      name: form.value.name.trim(),
      description: form.value.description || '',
      position: form.value.position || 0,
      is_temporary: form.value.is_temporary,
      max_users: form.value.max_users || 0,
    })
    emit('created', ch)
    model.value = false
  } catch (e) {
    error.value = e.message || 'Failed to create channel'
  } finally {
    loading.value = false
  }
}
</script>
