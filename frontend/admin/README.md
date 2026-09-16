# Admin user management

This no-build dashboard uses the server-side administration API. In production, serve `frontend/admin` from the same origin as the Go API, or configure the reverse proxy so `/api/v1/admin` reaches the API service. The browser uses the default API base URL `/api/v1/admin`.

Start the Go API with its normal environment and migrations applied, then serve the dashboard through that same origin. For a local static-server session against a separately running API, define a development-only base URL before `api.js` is loaded:

```html
<script>window.ADMIN_API_BASE_URL = 'http://127.0.0.1:8080/api/v1/admin';</script>
<script src="api.js"></script>
```

The API must allow that local origin during development. Do not use this cross-origin setup in production when a same-origin reverse proxy is available.

Seed an administrator in the backend environment with `GO_SERVICE_ADMIN_USERNAME` and `GO_SERVICE_ADMIN_PASSWORD`, then use that account at the login gate. Keep real values in the environment or secret manager; this repository intentionally contains no administrator credentials.

`AppSecret` stays backend-only. The dashboard stores only the current browser session's access and refresh tokens in `sessionStorage`; it never stores profile payloads, provider tokens, or passwords.

Run the frontend checks from the repository root:

```powershell
node frontend/admin/tools/api_adapter_test.js
node frontend/admin/tools/smoke_test.js
node --check frontend/admin/app.js
```
