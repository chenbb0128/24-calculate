import { login as adminLogin, logout as adminLogout } from '@/api/admin'
import { getToken, setToken, removeToken } from '@/utils/auth'
import { resetRouter } from '@/router'

const getDefaultState = () => ({
  token: getToken(),
  name: getToken() ? '管理员' : '',
  avatar: ''
})

const state = getDefaultState()

const mutations = {
  RESET_STATE: state => Object.assign(state, getDefaultState()),
  SET_TOKEN: (state, token) => { state.token = token },
  SET_NAME: (state, name) => { state.name = name },
  SET_AVATAR: (state, avatar) => { state.avatar = avatar }
}

const actions = {
  async login({ commit }, credentials) {
    const data = await adminLogin({
      username: String(credentials.username || '').trim(),
      password: String(credentials.password || '')
    })
    setToken(data.access_token)
    commit('SET_TOKEN', data.access_token)
    commit('SET_NAME', '管理员')
  },

  async getInfo({ commit }) {
    if (!getToken()) throw new Error('登录已失效，请重新登录')
    commit('SET_NAME', '管理员')
    return { name: '管理员', avatar: '' }
  },

  async logout({ commit }) {
    try {
      await adminLogout()
    } finally {
      removeToken()
      resetRouter()
      commit('RESET_STATE')
    }
  },

  resetToken({ commit }) {
    removeToken()
    commit('RESET_STATE')
    return Promise.resolve()
  }
}

export default { namespaced: true, state, mutations, actions }
