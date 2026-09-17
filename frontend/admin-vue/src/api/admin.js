const ACCESS_TOKEN_KEY = 'sanhuo.admin.access_token.v1'
const REFRESH_TOKEN_KEY = 'sanhuo.admin.refresh_token.v1'

export const MODERATION_MESSAGES = {
  NICKNAME_REJECTED: '昵称未通过审核',
  AVATAR_REJECTED: '头像未通过审核',
  MODERATION_PENDING: '资料审核尚未完成',
  NICKNAME_EDIT_DISABLED: '暂不支持修改昵称'
}

export class AdminApiError extends Error {
  constructor(status, envelope) {
    const body = envelope && typeof envelope === 'object' ? envelope : {}
    const businessCode = typeof body.business_code === 'string'
      ? body.business_code
      : typeof body.code === 'string' ? body.code : undefined
    super(MODERATION_MESSAGES[businessCode] || body.message || '请求失败')
    this.name = 'AdminApiError'
    this.status = Number.isInteger(status) ? status : 0
    this.code = typeof body.code === 'number' ? body.code : 0
    this.businessCode = businessCode
    this.isAuthError = this.status === 401
  }
}

function getStorage() {
  return typeof sessionStorage === 'undefined' ? null : sessionStorage
}

function getBaseUrl() {
  return String(process.env.VUE_APP_ADMIN_API || '/api/v1/admin').replace(/\/+$/, '')
}

function numericId(id) {
  const value = String(id)
  if (!/^[1-9]\d*$/.test(value) || !Number.isSafeInteger(Number(value))) {
    throw new AdminApiError(400, { message: '用户 ID 无效' })
  }
  return value
}

class AdminApi {
  constructor() {
    this.accessToken = getStorage() ? getStorage().getItem(ACCESS_TOKEN_KEY) || '' : ''
    this.refreshToken = getStorage() ? getStorage().getItem(REFRESH_TOKEN_KEY) || '' : ''
  }

  clearSession() {
    this.accessToken = ''
    this.refreshToken = ''
    if (getStorage()) {
      getStorage().removeItem(ACCESS_TOKEN_KEY)
      getStorage().removeItem(REFRESH_TOKEN_KEY)
    }
  }

  storeSession(tokens) {
    if (!tokens || !tokens.access_token || !tokens.refresh_token) {
      this.clearSession()
      throw new AdminApiError(0, { message: '登录会话无效' })
    }
    this.accessToken = tokens.access_token
    this.refreshToken = tokens.refresh_token
    if (getStorage()) {
      getStorage().setItem(ACCESS_TOKEN_KEY, this.accessToken)
      getStorage().setItem(REFRESH_TOKEN_KEY, this.refreshToken)
    }
  }

  async send(path, options = {}) {
    const headers = { ...(options.headers || {}) }
    if (options.auth) {
      if (!this.accessToken) throw new AdminApiError(401, { message: '登录已失效' })
      headers.Authorization = `Bearer ${this.accessToken}`
    }
    const response = await fetch(`${getBaseUrl()}${path}`, {
      method: options.method || 'GET',
      headers,
      body: options.body
    })
    let envelope = null
    try { envelope = await response.json() } catch (error) { /* empty response */ }
    if (!response.ok) throw new AdminApiError(response.status, envelope)
    return envelope && typeof envelope === 'object' ? envelope.data : undefined
  }

  async refresh() {
    if (!this.refreshToken) {
      this.clearSession()
      throw new AdminApiError(401, { message: '登录已失效' })
    }
    try {
      const data = await this.send('/auth/refresh', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: this.refreshToken })
      })
      this.storeSession(data)
      return data
    } catch (error) {
      this.clearSession()
      if (error instanceof AdminApiError) error.isAuthError = true
      throw error
    }
  }

  async authenticated(path, options = {}) {
    try {
      return await this.send(path, { ...options, auth: true })
    } catch (error) {
      if (!(error instanceof AdminApiError) || error.status !== 401) throw error
      await this.refresh()
      return this.send(path, { ...options, auth: true })
    }
  }

  login(credentials) {
    return this.send('/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(credentials)
    }).then(data => {
      this.storeSession(data)
      return data
    })
  }

  logout() {
    const token = this.refreshToken
    if (!token) {
      this.clearSession()
      return Promise.resolve()
    }
    return this.send('/auth/logout', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: token })
    }).finally(() => this.clearSession())
  }

  listUsers(filters = {}) {
    const query = new URLSearchParams({
      q: String(filters.query || ''),
      status: String(filters.status || 'all'),
      platform: String(filters.platform || 'all'),
      page: String(Math.max(1, Number(filters.page) || 1)),
      page_size: String(Math.max(1, Number(filters.pageSize) || 20))
    })
    return this.authenticated(`/users?${query.toString()}`)
  }

  getUser(id) { return this.authenticated(`/users/${numericId(id)}`) }
  disableUser(id) { return this.authenticated(`/users/${numericId(id)}/disable`, { method: 'POST' }) }
  enableUser(id) { return this.authenticated(`/users/${numericId(id)}/enable`, { method: 'POST' }) }
}

export const adminApi = new AdminApi()
export const login = credentials => adminApi.login(credentials)
export const logout = () => adminApi.logout()
export const listUsers = filters => adminApi.listUsers(filters)
export const getUser = id => adminApi.getUser(id)
export const disableUser = id => adminApi.disableUser(id)
export const enableUser = id => adminApi.enableUser(id)
