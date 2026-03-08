<template>
  <v-btn
    variant="text"
    class="back-nav mr-2"
    size="small"
    icon="mdi-arrow-left"
    title="Back"
    @click="goBack"
  />
</template>

<script setup>
import { useRouter } from 'vue-router'

const props = defineProps({
  /** Fallback route when there is no history to go back to */
  fallback: {
    type: String,
    default: null,
  },
})

const router = useRouter()

function goBack() {
  if (window.history.length > 1) {
    router.back()
  } else if (props.fallback) {
    router.push(props.fallback)
  } else {
    router.back()
  }
}
</script>
