# Admin user management demo

This is a no-build static demo for the admin user-management handoff.

Start the page with:

```powershell
cd D:\微信小游戏
python -m http.server 4173 --directory frontend/admin
```

Then open `http://127.0.0.1:4173/` in a browser.

Run the automated check with:

```powershell
node frontend/admin/tools/smoke_test.js
```

## Future data adapter boundary

The current page uses the demo seed data and browser `localStorage` through the `loadUsers()` and `saveUsers()` boundary in `app.js`. A later integration can replace that boundary with an authenticated user-data adapter while keeping the current filtering, pagination, drawer, and status-action UI contract.

This page uses demo data and does not contain administrator credentials, real API calls, or WeChat AppSecret.
