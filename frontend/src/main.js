import { createApp } from 'vue'
import App from './App.vue'
import { registerPlugins } from '@/plugins'
import store from '@/store'

const app = createApp(App)
registerPlugins(app)

;(async () => {
  await store.dispatch('auth/initAuth')
  app.mount('#app')
})()
