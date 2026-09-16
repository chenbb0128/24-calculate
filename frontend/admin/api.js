(function attachAdminApi(root) {
  'use strict';

  const ACCESS_TOKEN_KEY = 'sanhuo.admin.access_token.v1';
  const REFRESH_TOKEN_KEY = 'sanhuo.admin.refresh_token.v1';
  const MODERATION_MESSAGES = {
    NICKNAME_REJECTED: '昵称未通过审核',
    AVATAR_REJECTED: '头像未通过审核',
    MODERATION_PENDING: '资料审核尚未完成',
    NICKNAME_EDIT_DISABLED: '暂不支持修改昵称'
  };

  class AdminApiError extends Error {
    constructor(status, envelope) {
      const body = envelope && typeof envelope === 'object' ? envelope : {};
      const businessCode = typeof body.business_code === 'string'
        ? body.business_code
        : typeof body.code === 'string' ? body.code : undefined;
      const message = MODERATION_MESSAGES[businessCode]
        || (typeof body.message === 'string' && body.message ? body.message : '请求失败');
      super(message);
      this.name = 'AdminApiError';
      this.status = Number.isInteger(status) ? status : 0;
      this.code = typeof body.code === 'number' ? body.code : 0;
      this.businessCode = businessCode;
      this.isAuthError = this.status === 401;
    }
  }

  function normalizeBaseUrl(baseUrl) {
    const value = String(baseUrl || '/api/v1/admin').replace(/\/+$/, '');
    return value || '/api/v1/admin';
  }

  function numericId(id) {
    const value = String(id);
    if (!/^[1-9]\d*$/.test(value) || !Number.isSafeInteger(Number(value))) {
      throw new AdminApiError(400, { code: 0, message: '用户 ID 无效' });
    }
    return value;
  }

  function createAdminApi(options) {
    const config = options || {};
    const baseUrl = normalizeBaseUrl(config.baseUrl);
    const fetchImpl = config.fetchImpl || (root && root.fetch);
    const storage = config.storage || (root && root.sessionStorage);
    if (typeof fetchImpl !== 'function') throw new Error('fetch 不可用');
    if (!storage || typeof storage.getItem !== 'function') throw new Error('sessionStorage 不可用');

    let accessToken = storage.getItem(ACCESS_TOKEN_KEY) || '';
    let refreshToken = storage.getItem(REFRESH_TOKEN_KEY) || '';

    function clearSession() {
      accessToken = '';
      refreshToken = '';
      storage.removeItem(ACCESS_TOKEN_KEY);
      storage.removeItem(REFRESH_TOKEN_KEY);
    }

    function storeSession(tokens) {
      if (!tokens || typeof tokens.access_token !== 'string' || typeof tokens.refresh_token !== 'string'
        || !tokens.access_token || !tokens.refresh_token) {
        clearSession();
        throw new AdminApiError(0, { code: 0, message: '登录会话无效' });
      }
      accessToken = tokens.access_token;
      refreshToken = tokens.refresh_token;
      storage.setItem(ACCESS_TOKEN_KEY, accessToken);
      storage.setItem(REFRESH_TOKEN_KEY, refreshToken);
    }

    async function parseResponse(response) {
      try {
        return await response.json();
      } catch (error) {
        return null;
      }
    }

    async function send(path, options) {
      const requestOptions = options || {};
      const headers = { ...(requestOptions.headers || {}) };
      if (requestOptions.auth) {
        if (!accessToken) throw new AdminApiError(401, { code: 0, message: '登录已失效' });
        headers.Authorization = `Bearer ${accessToken}`;
      }
      const response = await fetchImpl(`${baseUrl}${path}`, {
        method: requestOptions.method || 'GET',
        headers,
        body: requestOptions.body
      });
      const envelope = await parseResponse(response);
      if (!response.ok) throw new AdminApiError(response.status, envelope);
      return envelope && typeof envelope === 'object' ? envelope.data : undefined;
    }

    async function refresh() {
      if (!refreshToken) {
        clearSession();
        throw new AdminApiError(401, { code: 0, message: '登录已失效' });
      }
      try {
        const data = await send('/auth/refresh', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ refresh_token: refreshToken })
        });
        storeSession(data);
        return data;
      } catch (error) {
        clearSession();
        if (error instanceof AdminApiError) {
          error.isAuthError = true;
          throw error;
        }
        throw error;
      }
    }

    async function authenticated(path, requestOptions) {
      try {
        return await send(path, { ...requestOptions, auth: true });
      } catch (error) {
        if (!(error instanceof AdminApiError) || error.status !== 401) throw error;
      }
      try {
        await refresh();
        return await send(path, { ...requestOptions, auth: true });
      } catch (error) {
        if (error instanceof AdminApiError && error.status === 401) {
          clearSession();
          error.isAuthError = true;
        }
        throw error;
      }
    }

    return {
      hasAccessToken() { return Boolean(accessToken); },
      clearSession,
      async login(username, password) {
        const data = await send('/auth/login', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ username, password })
        });
        storeSession(data);
        return data;
      },
      refresh,
      async logout() {
        const token = refreshToken;
        try {
          if (!token) return { revoked: false };
          return await send('/auth/logout', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ refresh_token: token })
          });
        } finally {
          clearSession();
        }
      },
      listUsers(filters) {
        const values = filters || {};
        const query = new URLSearchParams({
          q: String(values.query || ''),
          status: String(values.status || 'all'),
          platform: String(values.platform || 'all'),
          page: String(Math.max(1, Number(values.page) || 1)),
          page_size: String(Math.max(1, Number(values.pageSize) || 20))
        });
        return authenticated(`/users?${query.toString()}`);
      },
      getUser(id) {
        return authenticated(`/users/${numericId(id)}`);
      },
      disableUser(id) {
        return authenticated(`/users/${numericId(id)}/disable`, { method: 'POST' });
      },
      enableUser(id) {
        return authenticated(`/users/${numericId(id)}/enable`, { method: 'POST' });
      }
    };
  }

  const exported = { createAdminApi, AdminApiError };
  if (typeof module !== 'undefined' && module.exports) module.exports = exported;
  if (root) root.AdminApi = exported;
}(typeof window !== 'undefined' ? window : globalThis));
