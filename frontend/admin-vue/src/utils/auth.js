const AccessTokenKey = 'sanhuo.admin.access_token.v1'
const RefreshTokenKey = 'sanhuo.admin.refresh_token.v1'

function storage() {
  if (typeof sessionStorage === 'undefined') return null
  return sessionStorage
}

export function getToken() {
  return storage() ? storage().getItem(AccessTokenKey) || '' : ''
}

export function setToken(token) {
  if (storage()) storage().setItem(AccessTokenKey, token)
  return token
}

export function removeToken() {
  if (storage()) {
    storage().removeItem(AccessTokenKey)
    storage().removeItem(RefreshTokenKey)
  }
}
