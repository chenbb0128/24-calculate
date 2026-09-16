const VALID_STATUSES = new Set(['active', 'disabled']);

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
  return users.map((user) => selected.has(user.id) && VALID_STATUSES.has(status)
    ? { ...user, status }
    : { ...user });
}

function getStatusChangeTargets(users, ids, status) {
  if (!VALID_STATUSES.has(status)) return [];
  const selected = new Set(Array.isArray(ids) ? ids : []);
  const usersById = new Map(users.map((user) => [user.id, user]));
  return [...selected].filter((id) => {
    const user = usersById.get(id);
    return Boolean(user && user.status !== status
      && (status !== 'disabled' || user.status === 'active'));
  });
}

function getStats(users) {
  const list = Array.isArray(users) ? users : [];
  return {
    total: list.length,
    newToday: 0,
    active: list.filter((user) => user.status === 'active').length,
    disabled: list.filter((user) => user.status === 'disabled').length
  };
}

function formatDate(iso) {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return '--';
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', hour12: false
  }).format(date);
}

function getInitials(nickname) {
  const text = String(nickname || '').trim();
  if (!text) return '--';
  return text.length <= 2 ? text : `${text.slice(0, 1)}${text.slice(-1)}`;
}

function mapPlatform(platform) {
  return { wechat: '微信', taptap: 'TapTap', password: '账号' }[platform] || '账号';
}

function mapStatus(status) {
  return Number(status) === 0 || status === 'disabled' ? 'disabled' : 'active';
}

function mapServerUser(raw) {
  const user = raw || {};
  return {
    id: String(user.id || ''),
    username: String(user.username || '--'),
    nickname: String(user.nickname || '算术玩家'),
    avatar: String(user.avatar || ''),
    platform: mapPlatform(user.platform),
    status: mapStatus(user.status),
    createdAt: user.created_at || user.createdAt || '',
    lastActiveAt: user.last_active_at || user.updated_at || user.updatedAt || '',
    nicknameModerationStatus: user.nickname_moderation_status,
    avatarModerationStatus: user.avatar_moderation_status
  };
}

function moderationLabel(status) {
  return {
    approved: '已通过', passed: '已通过', pending: '审核中', rejected: '未通过',
    unavailable: '暂不可用', unreviewed: '待审核'
  }[String(status || '').toLowerCase()] || '未提供';
}

const state = {
  api: null, users: [], stats: { total: 0, new_today: 0, active: 0, disabled: 0 },
  query: '', status: 'all', platform: 'all', page: 1, pageSize: 20, total: 0,
  selectedIds: new Set(), detailId: null, detailUser: null, pendingAction: null,
  drawerReturnFocus: null, listRequest: 0
};

function getElement(id) {
  return document.getElementById(id);
}

function createTextElement(tagName, className, text) {
  const element = document.createElement(tagName);
  if (className) element.className = className;
  element.textContent = text;
  return element;
}

function getVisiblePage() {
  const totalPages = Math.max(1, Math.ceil(state.total / state.pageSize));
  return { page: state.page, pageSize: state.pageSize, total: state.total, totalPages, items: state.users };
}

function renderStats() {
  const container = getElement('statCards');
  if (!container) return;
  const stats = state.stats || {};
  const cards = [
    ['用户总数', stats.total || 0, '服务端统计', ''],
    ['今日新增', stats.new_today || 0, '按服务端时区统计', 'stat-card__trend--positive'],
    ['活跃用户', stats.active || 0, '当前正常状态', 'stat-card__trend--positive'],
    ['已禁用用户', stats.disabled || 0, '需要关注的账号', 'stat-card__trend--negative']
  ];
  const fragment = document.createDocumentFragment();
  for (const [label, value, trend, trendClass] of cards) {
    const card = createTextElement('article', 'stat-card', '');
    card.append(createTextElement('h2', '', label), createTextElement('strong', '', String(value)),
      createTextElement('span', `stat-card__trend ${trendClass}`.trim(), trend));
    fragment.append(card);
  }
  container.replaceChildren(fragment);
}

function createStatusTag(status) {
  const label = status === 'disabled' ? '已禁用' : '正常';
  return createTextElement('span', `status-tag status-tag--${VALID_STATUSES.has(status) ? status : 'active'}`, label);
}

function createPlatformChip(platform) {
  const platformClass = platform === '微信' ? 'wechat' : platform === 'TapTap' ? 'taptap' : '';
  return createTextElement('span', platformClass ? `platform-chip platform-chip--${platformClass}` : 'platform-chip', platform);
}

function createAvatar(user, large) {
  const avatar = createTextElement('span', `avatar${large ? ' avatar--large' : ''}`, '');
  const fallback = () => {
    avatar.classList.add('avatar--fallback');
    avatar.replaceChildren(createTextElement('span', 'avatar__initials', getInitials(user.nickname)));
  };
  if (!user.avatar) {
    fallback();
    return avatar;
  }
  const image = document.createElement('img');
  image.src = user.avatar;
  image.alt = `${user.nickname}头像`;
  image.addEventListener('error', fallback, { once: true });
  avatar.append(image);
  return avatar;
}

function createUserCell(user, selected) {
  const cell = createTextElement('td', '', '');
  const wrapper = createTextElement('div', 'user-cell', '');
  const checkbox = document.createElement('input');
  checkbox.type = 'checkbox';
  checkbox.checked = selected;
  checkbox.dataset.action = 'select-user';
  checkbox.dataset.userId = user.id;
  checkbox.setAttribute('aria-label', `选择${user.nickname}`);
  const details = createTextElement('div', '', '');
  details.append(createTextElement('div', 'user-name', user.nickname), createTextElement('div', 'user-meta', user.username));
  wrapper.append(checkbox, createAvatar(user, false), details);
  cell.append(wrapper);
  return cell;
}

function renderTable() {
  const body = getElement('userTableBody');
  if (!body) return;
  const view = getVisiblePage();
  const fragment = document.createDocumentFragment();
  if (!view.items.length) {
    const row = document.createElement('tr');
    const cell = createTextElement('td', 'empty-state', '');
    cell.colSpan = 6;
    cell.append(createTextElement('strong', '', '暂无匹配用户'), createTextElement('p', '', '请调整搜索词或筛选条件后重试。'));
    row.append(cell);
    fragment.append(row);
  } else {
    for (const user of view.items) {
      const selected = state.selectedIds.has(user.id);
      const row = document.createElement('tr');
      row.dataset.userId = user.id;
      row.className = selected ? 'is-selected' : '';
      row.setAttribute('aria-selected', String(selected));
      const platform = createTextElement('td', '', '');
      platform.append(createPlatformChip(user.platform));
      const status = createTextElement('td', '', '');
      status.append(createStatusTag(user.status));
      const actions = createTextElement('div', 'row-actions', '');
      const detail = createTextElement('button', 'row-action', '查看详情');
      detail.type = 'button';
      detail.dataset.action = 'detail';
      detail.dataset.userId = user.id;
      detail.setAttribute('aria-label', `查看${user.nickname}详情`);
      actions.append(detail);
      const actionCell = createTextElement('td', '', '');
      actionCell.append(actions);
      row.append(createUserCell(user, selected), platform, createTextElement('td', '', formatDate(user.createdAt)),
        createTextElement('td', '', formatDate(user.lastActiveAt)), status, actionCell);
      fragment.append(row);
    }
  }
  body.replaceChildren(fragment);
}

function renderPagination() {
  const container = getElement('pagination');
  if (!container) return;
  const view = getVisiblePage();
  const summary = createTextElement('p', 'pagination__summary', `第 ${view.page} / ${view.totalPages} 页，共 ${view.total} 位用户`);
  const controls = createTextElement('div', 'pagination__controls', '');
  for (const [label, page, disabled] of [['上一页', view.page - 1, view.page <= 1], ['下一页', view.page + 1, view.page >= view.totalPages]]) {
    const button = createTextElement('button', '', label);
    button.type = 'button';
    button.dataset.action = 'page';
    button.dataset.page = String(page);
    button.disabled = disabled;
    button.setAttribute('aria-label', label);
    controls.append(button);
  }
  container.replaceChildren(summary, controls);
}

function renderBulkBar() {
  const bar = getElement('batchActions');
  const count = getElement('selectedCount');
  if (!bar || !count) return;
  const existingIds = new Set(state.users.map((user) => user.id));
  state.selectedIds = new Set([...state.selectedIds].filter((id) => existingIds.has(id)));
  bar.hidden = state.selectedIds.size === 0;
  count.textContent = `已选择 ${state.selectedIds.size} 位用户`;
}

function ensureDrawerAction(drawer) {
  let footer = drawer.querySelector('.drawer-footer');
  if (!footer) {
    footer = createTextElement('div', 'drawer-footer', '');
    drawer.append(footer);
  }
  let button = footer.querySelector('[data-action="detail-status"]');
  if (!button) {
    button = createTextElement('button', 'primary-action', '');
    button.type = 'button';
    button.dataset.action = 'detail-status';
    footer.append(button);
  }
  return button;
}

function renderDrawer() {
  const drawer = getElement('detailDrawer');
  if (!drawer) return;
  const user = state.detailUser || state.users.find((item) => item.id === state.detailId);
  if (!user || state.detailId === null) {
    drawer.hidden = true;
    drawer.setAttribute('aria-hidden', 'true');
    return;
  }
  drawer.hidden = false;
  drawer.setAttribute('aria-hidden', 'false');
  getElement('detailNickname').textContent = user.nickname;
  getElement('detailIdentitySummary').textContent = `${user.nickname} · ${user.platform}用户`;
  getElement('detailStatus').replaceChildren(createStatusTag(user.status));
  getElement('detailAvatar').replaceChildren(createAvatar(user, true));
  getElement('detailUserId').textContent = user.id;
  getElement('detailUsername').textContent = user.username;
  getElement('detailPlatform').textContent = user.platform;
  getElement('detailCreatedAt').textContent = formatDate(user.createdAt);
  getElement('detailLastActiveAt').textContent = formatDate(user.lastActiveAt);
  getElement('detailNicknameModeration').textContent = moderationLabel(user.nicknameModerationStatus);
  getElement('detailAvatarModeration').textContent = moderationLabel(user.avatarModerationStatus);
  const action = ensureDrawerAction(drawer);
  action.dataset.userId = user.id;
  action.textContent = user.status === 'disabled' ? '启用账号' : '禁用账号';
  action.setAttribute('aria-label', `${action.textContent}${user.nickname}`);
}

function ensureSelectAllCheckbox() {
  const body = getElement('userTableBody');
  const header = body && body.closest('table') ? body.closest('table').querySelector('thead th:first-child') : null;
  if (!header) return null;
  let checkbox = header.querySelector('[data-action="select-all"]');
  if (!checkbox) {
    checkbox = document.createElement('input');
    checkbox.type = 'checkbox';
    checkbox.dataset.action = 'select-all';
    checkbox.setAttribute('aria-label', '选择当前页全部用户');
    header.prepend(checkbox);
  }
  return checkbox;
}

function syncSelectAll() {
  const checkbox = ensureSelectAllCheckbox();
  if (!checkbox) return;
  const visibleIds = state.users.map((user) => user.id);
  const selectedCount = visibleIds.filter((id) => state.selectedIds.has(id)).length;
  checkbox.checked = visibleIds.length > 0 && selectedCount === visibleIds.length;
  checkbox.indeterminate = selectedCount > 0 && selectedCount < visibleIds.length;
  checkbox.disabled = visibleIds.length === 0;
}

function renderAll() {
  renderStats();
  renderTable();
  renderPagination();
  renderBulkBar();
  renderDrawer();
  syncSelectAll();
}

function showToast(message, type) {
  const region = getElement('toastRegion');
  if (!region) return;
  const toast = createTextElement('div', `toast toast--${type || 'success'}`, message);
  toast.setAttribute('role', 'status');
  region.append(toast);
  setTimeout(() => toast.remove(), 3200);
}

function isDialogOpen(dialog) {
  return Boolean(dialog && (dialog.open || dialog.classList.contains('is-open')));
}

function closeConfirmation() {
  const dialog = getElement('confirmDialog');
  state.pendingAction = null;
  if (!dialog) return;
  if (typeof dialog.close === 'function') {
    if (dialog.open) dialog.close();
    dialog.hidden = false;
  } else {
    dialog.hidden = true;
    dialog.classList.remove('is-open');
    dialog.removeAttribute('open');
  }
}

function openConfirmation(count) {
  const dialog = getElement('confirmDialog');
  const message = getElement('confirmDialogMessage');
  if (!dialog || !message) return;
  message.textContent = `确定要禁用选中的 ${count} 个账号吗？`;
  if (typeof dialog.showModal === 'function') {
    dialog.hidden = false;
    if (!dialog.open) dialog.showModal();
  } else {
    dialog.hidden = false;
    dialog.classList.add('is-open');
    dialog.setAttribute('open', '');
  }
}

function closeDrawer() {
  const returnFocus = state.drawerReturnFocus;
  state.drawerReturnFocus = null;
  state.detailId = null;
  state.detailUser = null;
  renderDrawer();
  if (returnFocus && typeof returnFocus.focus === 'function') returnFocus.focus();
}

function showLogin() {
  const gate = getElement('loginGate');
  const app = getElement('app');
  if (gate) gate.hidden = false;
  if (app) {
    app.setAttribute('aria-hidden', 'true');
    app.setAttribute('inert', '');
  }
  getElement('loginPassword').value = '';
  getElement('loginUsername').focus();
}

function showDashboard() {
  const gate = getElement('loginGate');
  const app = getElement('app');
  if (gate) gate.hidden = true;
  if (app) {
    app.removeAttribute('aria-hidden');
    app.removeAttribute('inert');
  }
}

function handleApiError(error) {
  if (error && error.isAuthError) {
    state.api.clearSession();
    closeConfirmation();
    closeDrawer();
    showLogin();
    showToast('登录已失效，请重新登录', 'warning');
    return true;
  }
  showToast(error && error.message ? error.message : '请求失败，请稍后重试', 'error');
  return false;
}

async function loadUsers() {
  const request = ++state.listRequest;
  try {
    const data = await state.api.listUsers({
      query: state.query, status: state.status, platform: state.platform,
      page: state.page, pageSize: state.pageSize
    });
    if (request !== state.listRequest) return;
    state.users = Array.isArray(data.items) ? data.items.map(mapServerUser) : [];
    state.stats = data.stats || state.stats;
    state.page = Number(data.page) || state.page;
    state.pageSize = Number(data.page_size) || state.pageSize;
    state.total = Number(data.total) || 0;
    renderAll();
  } catch (error) {
    if (request === state.listRequest) handleApiError(error);
  }
}

async function openDrawer(id) {
  const listedUser = state.users.find((user) => user.id === id);
  if (!listedUser) return;
  state.drawerReturnFocus = document.activeElement && typeof document.activeElement.focus === 'function' ? document.activeElement : null;
  state.detailId = id;
  state.detailUser = listedUser;
  renderDrawer();
  const drawer = getElement('detailDrawer');
  if (drawer && typeof drawer.focus === 'function') drawer.focus();
  try {
    const detail = await state.api.getUser(id);
    if (state.detailId === id) {
      state.detailUser = mapServerUser(detail);
      renderDrawer();
    }
  } catch (error) {
    handleApiError(error);
  }
}

async function refreshOpenDetail() {
  if (state.detailId === null) return;
  try {
    state.detailUser = mapServerUser(await state.api.getUser(state.detailId));
    renderDrawer();
  } catch (error) {
    handleApiError(error);
  }
}

async function completeStatusChange(ids, status) {
  const targetIds = getStatusChangeTargets(state.users, ids, status);
  closeConfirmation();
  if (!targetIds.length) {
    showToast(status === 'disabled' ? '所选用户中没有可禁用的活跃账号，无需重复操作' : '没有找到可操作的用户', 'warning');
    return;
  }
  let completed = 0;
  const failures = [];
  for (const id of targetIds) {
    try {
      if (status === 'disabled') await state.api.disableUser(id);
      else await state.api.enableUser(id);
      completed += 1;
    } catch (error) {
      failures.push(error);
      if (handleApiError(error)) return;
    }
  }
  state.selectedIds = new Set([...state.selectedIds].filter((id) => !targetIds.includes(id)));
  await loadUsers();
  await refreshOpenDetail();
  if (failures.length) {
    showToast(`已完成 ${completed} 个账号操作，${failures.length} 个未完成`, 'warning');
  } else {
    showToast(status === 'disabled' ? `已禁用 ${completed} 个账号` : `已启用 ${completed} 个账号`, 'success');
  }
}

function requestStatusChange(ids, status) {
  const targetIds = getStatusChangeTargets(state.users, ids, status);
  if (!targetIds.length) {
    showToast(status === 'disabled' ? '所选用户中没有可禁用的活跃账号，无需重复操作' : '没有选择可操作的用户', 'warning');
    return;
  }
  if (status === 'disabled') {
    state.pendingAction = { ids: targetIds, status };
    openConfirmation(targetIds.length);
    return;
  }
  completeStatusChange(targetIds, status);
}

function confirmPendingAction() {
  if (!state.pendingAction) {
    closeConfirmation();
    return;
  }
  const action = state.pendingAction;
  completeStatusChange(action.ids, action.status);
}

function bindEvents() {
  const search = getElement('searchInput');
  const statusFilter = getElement('statusFilter');
  const platformFilter = getElement('platformFilter');
  const filters = getElement('userFilters');
  const table = getElement('userTableBody').closest('table');
  const pagination = getElement('pagination');
  const drawer = getElement('detailDrawer');
  const dialog = getElement('confirmDialog');
  const loginForm = getElement('loginForm');

  const applyFilters = () => {
    state.query = search.value;
    state.status = statusFilter.value;
    state.platform = platformFilter.value;
    state.page = 1;
    state.selectedIds.clear();
    loadUsers();
  };
  search.addEventListener('input', applyFilters);
  statusFilter.addEventListener('change', applyFilters);
  platformFilter.addEventListener('change', applyFilters);
  filters.addEventListener('submit', (event) => { event.preventDefault(); applyFilters(); });
  loginForm.addEventListener('submit', async (event) => {
    event.preventDefault();
    const button = getElement('loginSubmitButton');
    button.disabled = true;
    try {
      await state.api.login(getElement('loginUsername').value, getElement('loginPassword').value);
      showDashboard();
      state.page = 1;
      await loadUsers();
    } catch (error) {
      handleApiError(error);
    } finally {
      button.disabled = false;
    }
  });
  table.addEventListener('change', (event) => {
    const target = event.target;
    if (!target || !target.matches('input[type="checkbox"]')) return;
    if (target.dataset.action === 'select-all') {
      for (const user of state.users) {
        if (target.checked) state.selectedIds.add(user.id);
        else state.selectedIds.delete(user.id);
      }
      renderTable(); renderBulkBar(); syncSelectAll();
      return;
    }
    if (target.dataset.action === 'select-user') {
      if (target.checked) state.selectedIds.add(target.dataset.userId);
      else state.selectedIds.delete(target.dataset.userId);
      const row = target.closest('tr');
      if (row) {
        row.classList.toggle('is-selected', target.checked);
        row.setAttribute('aria-selected', String(target.checked));
      }
      renderBulkBar(); syncSelectAll();
    }
  });
  table.addEventListener('click', (event) => {
    const target = event.target;
    const button = target.closest('button[data-action="detail"]');
    if (button) return openDrawer(button.dataset.userId);
    if (target.closest('input, button, a')) return;
    const row = target.closest('tr[data-user-id]');
    if (row) openDrawer(row.dataset.userId);
  });
  pagination.addEventListener('click', (event) => {
    const button = event.target.closest('button[data-action="page"]');
    if (!button || button.disabled) return;
    state.page = Number(button.dataset.page) || 1;
    loadUsers();
  });
  getElement('refreshButton').addEventListener('click', () => { state.selectedIds.clear(); loadUsers(); });
  getElement('batchDisableButton').addEventListener('click', () => {
    if (!state.selectedIds.size) return showToast('请先选择要禁用的用户', 'warning');
    requestStatusChange([...state.selectedIds], 'disabled');
  });
  getElement('closeDetailButton').addEventListener('click', closeDrawer);
  drawer.addEventListener('click', (event) => {
    const button = event.target.closest('button[data-action="detail-status"]');
    if (!button) return;
    const user = state.detailUser || state.users.find((item) => item.id === button.dataset.userId);
    if (user) requestStatusChange([user.id], user.status === 'disabled' ? 'active' : 'disabled');
  });
  dialog.querySelector('form').addEventListener('submit', (event) => {
    event.preventDefault();
    if (event.submitter && event.submitter.value === 'confirm') confirmPendingAction();
    else closeConfirmation();
  });
  dialog.addEventListener('cancel', (event) => { event.preventDefault(); closeConfirmation(); });
  dialog.addEventListener('click', (event) => { if (event.target === dialog) closeConfirmation(); });
  document.addEventListener('keydown', (event) => {
    if (event.key !== 'Escape') return;
    if (isDialogOpen(dialog)) { event.preventDefault(); closeConfirmation(); }
    else if (state.detailId !== null) { event.preventDefault(); closeDrawer(); }
  });
}

async function boot() {
  if (document.documentElement.dataset.adminBooted === 'true') return;
  document.documentElement.dataset.adminBooted = 'true';
  const dialog = getElement('confirmDialog');
  if (dialog && typeof dialog.showModal !== 'function') dialog.hidden = true;
  state.api = window.AdminApi.createAdminApi({ baseUrl: window.ADMIN_API_BASE_URL || '/api/v1/admin' });
  ensureSelectAllCheckbox();
  bindEvents();
  if (state.api.hasAccessToken()) {
    showDashboard();
    await loadUsers();
    return;
  }
  try {
    await state.api.refresh();
    showDashboard();
    await loadUsers();
  } catch (error) {
    showLogin();
  }
}

const AdminCore = { filterUsers, paginateUsers, setUserStatus, getStatusChangeTargets, getStats, formatDate, getInitials, mapServerUser, moderationLabel };
if (typeof module !== 'undefined' && module.exports) module.exports = AdminCore;
if (typeof window !== 'undefined') window.AdminCore = AdminCore;
if (typeof document !== 'undefined') {
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot, { once: true });
  else boot();
}
