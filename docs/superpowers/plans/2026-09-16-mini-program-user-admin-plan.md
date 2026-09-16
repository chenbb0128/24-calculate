# 小程序用户管理后台 MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改动现有小游戏和 Go 服务的前提下，制作一个零依赖、可直接运行的小程序用户管理后台前端 MVP。

**Architecture:** 在 `frontend/admin/` 建立独立静态应用。`app.js` 同时提供可被 Node smoke test 加载的纯数据/状态函数，以及在浏览器中运行的 DOM 渲染和事件绑定；`index.html` 只负责语义化页面骨架，`styles.css` 负责桌面优先布局、视觉令牌和窄屏适配。演示用户状态通过 `localStorage` 持久化，后续接入真实后端时只替换数据 adapter。

**Tech Stack:** 原生 HTML、CSS、JavaScript；Node.js 内置 `assert` 和 `fs`；不引入 npm 依赖或构建工具。

**Spec:** `docs/superpowers/specs/2026-09-16-mini-program-user-admin-design.md`

## Global Constraints

- 只创建 `frontend/admin/`，不修改现有 `frontend/wechat_game/`、`frontend/taptap_game/` 和 `backend/`。
- 页面使用中文文案，产品标题为“ 三火算术 · 运营后台”。
- 演示数据不代表生产数据；不加入管理员登录、权限、真实 API、MySQL 或微信 AppSecret。
- localStorage 命名空间固定为 `sanhuo.admin.users.v1`，读取失败必须回退内置演示数据。
- 用户状态只能是 `active` 或 `disabled`；审核业务码属于后续真实后端阶段，不在本阶段实现。
- 所有可操作控件提供键盘 focus 状态和可读名称，状态不能只依赖颜色表达。
- 每个行为先写失败测试，确认失败后再实现；每个独立任务结束后运行对应检查并提交本任务文件。

---

## 文件结构

- Create: `frontend/admin/index.html` — 页面骨架、导航、统计卡片、筛选工具栏、表格、抽屉、确认弹窗和 toast 容器。
- Create: `frontend/admin/styles.css` — 颜色/间距/圆角令牌、布局、表格、状态标签、抽屉、弹窗和窄屏规则。
- Create: `frontend/admin/app.js` — 18 条确定性演示数据、纯函数、localStorage adapter、DOM 渲染和事件处理。
- Create: `frontend/admin/tools/smoke_test.js` — Node 内置断言测试，同时检查 HTML 必备挂载点。

## Task 1: 建立用户数据核心与 smoke test

**Files:**
- Create: `frontend/admin/tools/smoke_test.js`
- Create: `frontend/admin/app.js`

**Interfaces:**
- Produces `filterUsers(users, filters) -> User[]`。
- Produces `paginateUsers(users, page, pageSize) -> { page, pageSize, total, totalPages, items }`。
- Produces `setUserStatus(users, ids, status) -> User[]`。
- Produces `serializeUserState(users) -> string`。
- Produces `restoreUserState(raw, fallbackUsers) -> User[]`。
- Produces `AdminCore` through CommonJS `module.exports` and browser `window.AdminCore`.

- [ ] **Step 1: Write the failing tests for filtering and pagination**

在 `smoke_test.js` 中先写最小测试入口和以下断言；此时 `app.js` 不存在，测试应因模块缺失而失败：

```js
const assert = require('node:assert/strict');
const { filterUsers, paginateUsers, setUserStatus, serializeUserState, restoreUserState } = require('../app');

const users = [
  { id: 'U1001', username: 'wx_1001', nickname: '晴天小猫', platform: '微信', status: 'active' },
  { id: 'U1002', username: 'tap_1002', nickname: '夜航星', platform: 'TapTap', status: 'disabled' },
  { id: 'U1003', username: 'wx_1003', nickname: '小火花', platform: '微信', status: 'active' }
];

assert.equal(filterUsers(users, { query: '晴天', status: 'all', platform: 'all' }).length, 1);
assert.equal(filterUsers(users, { query: 'U1002', status: 'all', platform: 'all' })[0].id, 'U1002');
assert.equal(filterUsers(users, { query: '', status: 'active', platform: '微信' }).length, 2);
assert.equal(filterUsers(users, { query: '不存在', status: 'all', platform: 'all' }).length, 0);

const page = paginateUsers(users, 2, 2);
assert.deepEqual(page, { page: 2, pageSize: 2, total: 3, totalPages: 2, items: [users[2]] });
assert.equal(paginateUsers(users, 9, 2).page, 2);
```

- [ ] **Step 2: Run the test and verify the failure is about the missing implementation**

Run from `D:\微信小游戏`:

```powershell
node frontend/admin/tools/smoke_test.js
```

Expected: FAIL with a module-not-found error for `frontend/admin/app.js`, not a syntax error in the test.

- [ ] **Step 3: Write the minimal pure data implementation**

在 `app.js` 顶部定义以下接口，所有函数只返回新数组/新对象，不直接修改传入的 `users`：

```js
function filterUsers(users, filters) {
  const query = String(filters.query || '').trim().toLowerCase();
  return users.filter((user) => {
    const matchesQuery = !query || [user.id, user.username, user.nickname]
      .some((value) => String(value).toLowerCase().includes(query));
    const matchesStatus = filters.status === 'all' || user.status === filters.status;
    const matchesPlatform = filters.platform === 'all' || user.platform === filters.platform;
    return matchesQuery && matchesStatus && matchesPlatform;
  });
}

function paginateUsers(users, page, pageSize) {
  const safePageSize = Math.max(1, Number(pageSize) || 1);
  const totalPages = Math.max(1, Math.ceil(users.length / safePageSize));
  const safePage = Math.min(Math.max(1, Number(page) || 1), totalPages);
  const start = (safePage - 1) * safePageSize;
  return { page: safePage, pageSize: safePageSize, total: users.length,
    totalPages, items: users.slice(start, start + safePageSize) };
}

function setUserStatus(users, ids, status) {
  const selected = new Set(ids);
  return users.map((user) => selected.has(user.id) ? { ...user, status } : { ...user });
}
```

`serializeUserState` 只保留 `id` 与 `status`，`restoreUserState` 只接受数组并按 fallback 的用户 ID 合并状态；无效 JSON、未知 ID 或未知 status 都回退到 fallback 对应值。

- [ ] **Step 4: Add persistence and status tests before implementation**

在测试文件中追加：

```js
const changed = setUserStatus(users, ['U1001', 'U1003'], 'disabled');
assert.equal(changed[0].status, 'disabled');
assert.equal(changed[1].status, 'disabled');
assert.equal(changed[2].status, 'disabled');
assert.equal(users[0].status, 'active');

const restored = restoreUserState(serializeUserState(changed), users);
assert.deepEqual(restored.map((user) => user.status), ['disabled', 'disabled', 'disabled']);
assert.equal(restoreUserState('{broken', users)[0].status, 'active');
assert.equal(restoreUserState(JSON.stringify([{ id: 'UNKNOWN', status: 'disabled' }]), users)[0].status, 'active');
```

Run the same command and expect a failure because the persistence functions are not defined yet.

- [ ] **Step 5: Implement persistence, exports, and deterministic seed data**

Use `const STORAGE_KEY = 'sanhuo.admin.users.v1'`. Provide 18 records covering both `微信` and `TapTap`, both statuses, and these fields: `id`, `username`, `nickname`, `avatar`, `platform`, `status`, `createdAt`, `lastActiveAt`, `sessions`, `totalMatches`. Use deterministic date strings around `2026-09-16`, and do not use `Math.random()`.

Export the five core functions plus `STORAGE_KEY` and `SEED_USERS`:

```js
const AdminCore = { STORAGE_KEY, SEED_USERS, filterUsers, paginateUsers,
  setUserStatus, serializeUserState, restoreUserState };
if (typeof module !== 'undefined' && module.exports) module.exports = AdminCore;
if (typeof window !== 'undefined') window.AdminCore = AdminCore;
```

- [ ] **Step 6: Run the core tests and commit**

Run:

```powershell
node frontend/admin/tools/smoke_test.js
```

Expected: PASS for search, combined filters, empty results, bounded pagination, immutable status updates, and safe persistence fallback.

Commit only these new files:

```powershell
git add frontend/admin/app.js frontend/admin/tools/smoke_test.js
git commit -m "feat: add admin user data core"
```

## Task 2: Build the semantic page shell

**Files:**
- Modify: `frontend/admin/tools/smoke_test.js`
- Create: `frontend/admin/index.html`

**Interfaces:**
- Consumes `AdminCore` from `app.js`.
- Produces stable DOM hooks: `#app`, `#sidebar`, `#statCards`, `#searchInput`, `#statusFilter`, `#platformFilter`, `#userTableBody`, `#pagination`, `#detailDrawer`, `#confirmDialog`, `#toastRegion`.

- [ ] **Step 1: Add a failing HTML contract test**

在 smoke test 中用 `node:fs` 读取 `../index.html`，断言文档包含以下字符串：

```js
const fs = require('node:fs');
const html = fs.readFileSync(require('node:path').join(__dirname, '..', 'index.html'), 'utf8');
for (const id of ['app', 'statCards', 'searchInput', 'statusFilter', 'platformFilter',
  'userTableBody', 'pagination', 'detailDrawer', 'confirmDialog', 'toastRegion']) {
  assert.match(html, new RegExp(`id=["']${id}["']`));
}
assert.match(html, /app\.js/);
assert.match(html, /styles\.css/);
```

Run the test; expected failure should say the HTML file is missing.

- [ ] **Step 2: Write the minimal semantic HTML shell**

创建 `<!doctype html>` 页面，包含：

- 侧栏品牌、导航项“概览 / 用户管理 / 内容审核 / 运营设置”，只让“用户管理”可用并高亮。
- 主内容标题“用户管理”和辅助说明“管理小程序用户资料与账号状态”。
- 统计卡片容器 `#statCards`。
- 搜索输入、状态下拉、平台下拉、刷新按钮和批量操作栏。
- `table` 包含 `thead` 与空的 `tbody#userTableBody`，表头使用“用户 / 平台 / 注册时间 / 最近活跃 / 状态 / 操作”。
- `#pagination` 分页容器、`aside#detailDrawer` 详情抽屉、`dialog#confirmDialog` 确认弹窗、`div#toastRegion` `aria-live="polite"` 提示区域。
- 页面底部加载顺序为 `styles.css`、`app.js`；按钮和输入框都使用真实 HTML 控件，不用 div 模拟按钮。

- [ ] **Step 3: Run the contract test and commit**

Run:

```powershell
node frontend/admin/tools/smoke_test.js
```

Expected: PASS for the HTML hook assertions. Commit:

```powershell
git add frontend/admin/index.html frontend/admin/tools/smoke_test.js
git commit -m "feat: add admin dashboard page shell"
```

## Task 3: Add the visual system and responsive layout

**Files:**
- Create: `frontend/admin/styles.css`

**Interfaces:**
- Consumes the class names and IDs from `index.html`.
- Produces a desktop dashboard at 1280px and a usable single-column layout below 860px.

- [ ] **Step 1: Define the visual tokens and base layout**

Define CSS custom properties for `--ink`, `--muted`, `--line`, `--surface`, `--canvas`, `--brand`, `--brand-soft`, `--success`, `--warning`, `--danger`, `--radius-md`, and `--shadow-card`. Use a deep navy sidebar, off-white canvas, white cards, and blue-green brand accent. Set `box-sizing: border-box`, system Chinese font stack, visible `:focus-visible`, and `prefers-reduced-motion` fallback.

- [ ] **Step 2: Style the dashboard components**

Implement grid layout for sidebar/main, stat cards with icon circles and trend copy, toolbar controls, table row spacing, avatar initials fallback, platform chips, active/disabled status tags with text, selected-row treatment, bulk action bar, right-side drawer, modal dialog, toast, empty state, and pagination buttons. Keep table content readable at 14–15px and use a minimum table width so narrow screens can scroll rather than collapse fields.

- [ ] **Step 3: Add the responsive rules and run static checks**

Below `860px`, shrink the sidebar to a compact rail or move it above the content, make the toolbar wrap, let `.table-scroll` scroll horizontally, and set the drawer width to `100%`. Below `560px`, stack stat cards, make the header actions full-width, and keep the primary status action visible.

Run:

```powershell
node frontend/admin/tools/smoke_test.js
```

Expected: existing tests remain PASS. Commit:

```powershell
git add frontend/admin/styles.css
git commit -m "feat: style admin dashboard responsive layout"
```

## Task 4: Wire browser state, rendering, and interactions

**Files:**
- Modify: `frontend/admin/app.js`
- Modify: `frontend/admin/tools/smoke_test.js`

**Interfaces:**
- Consumes the DOM hooks from `index.html` and the visual classes from `styles.css`.
- Uses `AdminCore` for filtering, pagination, status updates, and persistence.
- Produces browser behaviors: search, combined filters, select-all, pagination, detail drawer, single status action, batch disable, confirmation dialog, toast, refresh, and localStorage recovery.

- [ ] **Step 1: Add failing behavior checks for the new render helpers**

Add pure helper checks to smoke test for the functions that will be exported from `AdminCore`: `getStats(users)`, `formatDate(iso)`, and `getInitials(nickname)`. Required expectations:

```js
assert.deepEqual(getStats(users), { total: 3, newToday: 0, active: 2, disabled: 1 });
assert.equal(getInitials('晴天小猫'), '晴猫');
assert.match(formatDate('2026-09-16T09:18:00+08:00'), /2026/);
```

Run the smoke test and confirm it fails because these helpers are not yet exported.

- [ ] **Step 2: Implement pure display helpers and initial state**

Add `getStats`, `formatDate`, `getInitials`, and an internal state object:

```js
const state = {
  users: loadUsers(), query: '', status: 'all', platform: 'all',
  page: 1, pageSize: 6, selectedIds: new Set(), detailId: null,
  pendingAction: null
};
```

`getStats` counts `active`, `disabled`, and users whose `createdAt` falls on `2026-09-16` in China time for deterministic demo output. `loadUsers` catches storage errors and calls `restoreUserState`; `saveUsers` catches write errors without throwing into the UI.

- [ ] **Step 3: Implement render functions using one derived view model**

Add `getVisiblePage()` which calls `filterUsers` then `paginateUsers`, and implement:

- `renderStats()` updates `#statCards` with four cards.
- `renderTable()` rebuilds `#userTableBody`, escapes text with `textContent`, renders selected checkboxes, status tags, and a `查看详情` button.
- `renderPagination()` shows current page, total pages, previous/next controls, and disables boundary buttons.
- `renderBulkBar()` shows selected count and “批量禁用” only when selection is non-empty.
- `renderDrawer()` fills `#detailDrawer` with the selected user or closes it if the ID disappeared.
- `renderAll()` calls the five renderers and synchronizes the table select-all checkbox.

No user content may be inserted through `innerHTML` without first using fixed templates and `textContent` for user-provided fields.

- [ ] **Step 4: Implement event handlers and persistence**

Bind `input` on `#searchInput`, `change` on both filters, click events for row checkboxes, select-all, pagination, refresh, drawer close, single action, bulk action, dialog cancel/confirm, and keyboard Escape for drawer/dialog. Filter changes set `state.page = 1`; status changes call `setUserStatus`, save, clear completed selections, close the dialog, re-render, and show a toast. If bulk action is clicked with no selected IDs, show a toast and do not modify users.

Use a native `dialog` when available and a hidden/open class fallback when not; the confirm copy must say how many accounts will be disabled. Initial boot must be guarded by `if (typeof document !== 'undefined')` so Node can require the file without a DOM.

- [ ] **Step 5: Run the full smoke test and commit**

Run:

```powershell
node frontend/admin/tools/smoke_test.js
```

Expected: PASS for core behavior, helper behavior, HTML contract, and script loading. Commit:

```powershell
git add frontend/admin/app.js frontend/admin/tools/smoke_test.js
git commit -m "feat: wire admin user management interactions"
```

## Task 5: Browser verification and handoff notes

**Files:**
- Modify: `frontend/admin/tools/smoke_test.js` only if verification exposes a reproducible defect.
- Create: `frontend/admin/README.md`

**Interfaces:**
- Documents the no-build run command and the future data adapter boundary.

- [ ] **Step 1: Add the run instructions**

Create `frontend/admin/README.md` with these exact commands:

```powershell
cd D:\微信小游戏
python -m http.server 4173 --directory frontend/admin
```

Then open `http://127.0.0.1:4173/` in a browser. Also document the automated check:

```powershell
node frontend/admin/tools/smoke_test.js
```

State explicitly that this page uses demo data and does not contain administrator credentials, real API calls, or WeChat AppSecret.

- [ ] **Step 2: Run the automated verification**

Run:

```powershell
node frontend/admin/tools/smoke_test.js
```

Expected: process exit code `0`, all assertions pass, and no stack trace is printed.

- [ ] **Step 3: Perform the manual browser checklist**

With the static server running, verify in order:

1. Initial page shows four statistic cards and a populated user table.
2. Searching `晴天` leaves only matching users; clearing search restores the page.
3. Combining platform and status filters changes the count and resets to page 1.
4. Selecting the header checkbox selects only the current filtered page; the bulk bar reports the count.
5. Opening a row shows the detail drawer; disabling from the drawer requires confirmation and updates the row/card.
6. Batch disabling two users updates both rows and persists after reload.
7. A query with no matches shows the empty state; previous/next pagination buttons disable at boundaries.
8. At a narrow viewport, the toolbar wraps, the table scrolls horizontally, and the detail drawer remains usable.
9. Keyboard Tab reaches search, filters, row actions, drawer close, and dialog actions; Escape closes the open drawer/dialog.

- [ ] **Step 4: Commit handoff notes**

Run:

```powershell
git add frontend/admin/README.md
git commit -m "docs: add admin dashboard run guide"
```

Then inspect only the new admin files:

```powershell
git status --short -- frontend/admin docs/superpowers/plans/2026-09-16-mini-program-user-admin-plan.md
git diff HEAD~5..HEAD --stat -- frontend/admin
```

The final handoff must state the smoke-test result, the browser URL, and that the backend moderation checklist remains for a later phase.

## Coverage Check

- User list/search/filter/pagination: Task 1 core tests and Task 4 browser wiring.
- Detail drawer and single/batch status actions: Task 4.
- Demo persistence and corrupted storage fallback: Task 1 and Task 4.
- Empty state, feedback, responsive layout, accessibility hooks: Tasks 2–5.
- No backend/AppSecret changes: Global Constraints and Task 5 README.
- Future moderation business codes, historical nickname cleanup, migration, and `go test ./...`: captured in the design spec as a later backend acceptance checklist; deliberately not implemented by this plan.
