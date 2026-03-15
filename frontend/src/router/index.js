import { createRouter, createWebHistory } from 'vue-router'
import { routes } from 'vue-router/auto-routes'
import store from '@/store'
import { getIngressBase } from '@/utils/ingress'

function getRouterBase() {
  const base = getIngressBase()
  return base ? base + '/' : import.meta.env.BASE_URL
}

const router = createRouter({
  history: createWebHistory(getRouterBase()),
  routes: [...routes],
})

router.beforeEach(async (to) => {
  const isAuth = store.getters['auth/isAuthenticated']
  const isLogin = to.path === '/login'
  const isRegister = to.path === '/register'

  if (isLogin || isRegister) {
    if (isAuth) {
      return '/'
    }
    if (isLogin && !store.getters['auth/hasUsers']) {
      return '/register'
    }
    return
  }

  if (!isAuth) {
    return '/login'
  }

  if (to.path.startsWith('/admin') && !store.getters['auth/isAdmin']) {
    return '/'
  }
})

export default router
