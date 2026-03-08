const TOKEN_KEY = 'go-mumble-server:token'
const USER_KEY = 'go-mumble-server:user'

const state = {
  token: localStorage.getItem(TOKEN_KEY),
  user: JSON.parse(localStorage.getItem(USER_KEY) || 'null'),
  hasUsers: true,
}

const getters = {
  isAuthenticated: (state) => !!state.token,
  isGuest: (state) => !state.token,
  isAdmin: (state) => state.user?.role === 'admin',
  userRole: (state) => state.user?.role || null,
  user: (state) => state.user,
  token: (state) => state.token,
  hasUsers: (state) => state.hasUsers,
}

const mutations = {
  setAuth(state, { token, user }) {
    state.token = token
    state.user = user
    if (token) {
      localStorage.setItem(TOKEN_KEY, token)
    } else {
      localStorage.removeItem(TOKEN_KEY)
    }
    if (user) {
      localStorage.setItem(USER_KEY, JSON.stringify(user))
    } else {
      localStorage.removeItem(USER_KEY)
    }
  },
  clearAuth(state) {
    state.token = null
    state.user = null
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(USER_KEY)
  },
  setHasUsers(state, value) {
    state.hasUsers = value
  },
}

const actions = {
  async login({ commit }, { username, password }) {
    const res = await fetch('/api/v1/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    })
    const data = await res.json()
    if (!res.ok) {
      throw new Error(data.error || 'Login failed')
    }
    commit('setAuth', { token: data.token, user: data.user })
    commit('setHasUsers', true)
    return data
  },
  async register({ commit }, { username, password }) {
    const res = await fetch('/api/v1/auth/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    })
    const data = await res.json()
    if (!res.ok) {
      throw new Error(data.error || 'Registration failed')
    }
    commit('setAuth', { token: data.token, user: data.user })
    commit('setHasUsers', true)
    return data
  },
  async logout({ commit, dispatch }) {
    commit('clearAuth')
    await dispatch('checkAuthStatus')
  },
  async checkAuthStatus({ state, commit }) {
    const token = state.token
    const headers = { 'Content-Type': 'application/json' }
    if (token) {
      headers.Authorization = `Bearer ${token}`
    }
    try {
      const res = await fetch('/api/v1/auth/status', { headers })
      const data = await res.json()
      if (token) {
        if (res.status === 401) {
          commit('clearAuth')
          return { valid: false }
        }
        if (data.user) {
          commit('setAuth', { token, user: data.user })
          return { valid: true, user: data.user }
        }
        return { valid: false }
      }
      const hasUsers = data.hasUsers ?? true
      commit('setHasUsers', hasUsers)
      return { hasUsers }
    } catch (e) {
      if (token) {
        console.log('[auth/checkAuthStatus] Token validation failed:', e)
        commit('clearAuth')
        return { valid: false }
      }
      return { hasUsers: true }
    }
  },
  initFromStorage({ state, commit }) {
    if (state.token && state.user) {
      return
    }
    const token = localStorage.getItem(TOKEN_KEY)
    const user = JSON.parse(localStorage.getItem(USER_KEY) || 'null')
    if (token && user) {
      commit('setAuth', { token, user })
    }
  },
  async initAuth({ dispatch }) {
    await dispatch('initFromStorage')
    await dispatch('checkAuthStatus')
  },
}

export default {
  namespaced: true,
  state,
  getters,
  mutations,
  actions,
}
