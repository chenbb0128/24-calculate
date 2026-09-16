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

function serializeUserState(users) {
  return JSON.stringify(users
    .filter((user) => VALID_STATUSES.has(user.status))
    .map((user) => ({ id: user.id, status: user.status })));
}

function restoreUserState(raw, fallbackUsers) {
  const fallback = fallbackUsers.map((user) => ({ ...user }));
  let persisted;
  try {
    persisted = typeof raw === 'string' ? JSON.parse(raw) : null;
  } catch (error) {
    return fallback;
  }

  if (!Array.isArray(persisted)) return fallback;

  const persistedStatuses = new Map();
  for (const entry of persisted) {
    if (entry && typeof entry === 'object' && typeof entry.id === 'string') {
      persistedStatuses.set(entry.id, entry.status);
    }
  }

  return fallback.map((user) => {
    const savedStatus = persistedStatuses.get(user.id);
    const status = VALID_STATUSES.has(savedStatus)
      ? savedStatus
      : user.status;
    return { ...user, status };
  });
}

const STORAGE_KEY = 'sanhuo.admin.users.v1';
const DEMO_AVATAR = 'data:image/svg+xml,%3Csvg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 64 64%22%3E%3Ccircle cx=%2232%22 cy=%2232%22 r=%2232%22 fill=%22%23dbeafe%22/%3E%3C/svg%3E';
const SEED_USERS = [
  { id: 'U1001', username: 'wx_1001', nickname: '晴天小猫', avatar: DEMO_AVATAR, platform: '微信', status: 'active', createdAt: '2026-09-01T10:20:00+08:00', lastActiveAt: '2026-09-16T09:18:00+08:00', sessions: 128, totalMatches: 42 },
  { id: 'U1002', username: 'tap_1002', nickname: '夜航星', avatar: DEMO_AVATAR, platform: 'TapTap', status: 'disabled', createdAt: '2026-08-28T14:06:00+08:00', lastActiveAt: '2026-09-15T21:42:00+08:00', sessions: 64, totalMatches: 18 },
  { id: 'U1003', username: 'wx_1003', nickname: '小火花', avatar: DEMO_AVATAR, platform: '微信', status: 'active', createdAt: '2026-09-03T08:36:00+08:00', lastActiveAt: '2026-09-16T08:54:00+08:00', sessions: 96, totalMatches: 31 },
  { id: 'U1004', username: 'tap_1004', nickname: '山风', avatar: DEMO_AVATAR, platform: 'TapTap', status: 'active', createdAt: '2026-09-05T16:12:00+08:00', lastActiveAt: '2026-09-16T08:21:00+08:00', sessions: 77, totalMatches: 24 },
  { id: 'U1005', username: 'wx_1005', nickname: '白露', avatar: DEMO_AVATAR, platform: '微信', status: 'disabled', createdAt: '2026-09-06T11:48:00+08:00', lastActiveAt: '2026-09-14T19:30:00+08:00', sessions: 41, totalMatches: 12 },
  { id: 'U1006', username: 'tap_1006', nickname: '星河', avatar: DEMO_AVATAR, platform: 'TapTap', status: 'active', createdAt: '2026-09-08T09:25:00+08:00', lastActiveAt: '2026-09-16T07:58:00+08:00', sessions: 118, totalMatches: 39 },
  { id: 'U1007', username: 'wx_1007', nickname: '稻草人', avatar: DEMO_AVATAR, platform: '微信', status: 'active', createdAt: '2026-09-09T13:17:00+08:00', lastActiveAt: '2026-09-16T07:26:00+08:00', sessions: 52, totalMatches: 16 },
  { id: 'U1008', username: 'tap_1008', nickname: '云朵汽水', avatar: DEMO_AVATAR, platform: 'TapTap', status: 'disabled', createdAt: '2026-09-10T17:04:00+08:00', lastActiveAt: '2026-09-13T16:40:00+08:00', sessions: 33, totalMatches: 9 },
  { id: 'U1009', username: 'wx_1009', nickname: '小树懒', avatar: DEMO_AVATAR, platform: '微信', status: 'active', createdAt: '2026-09-11T07:42:00+08:00', lastActiveAt: '2026-09-16T06:52:00+08:00', sessions: 88, totalMatches: 27 },
  { id: 'U1010', username: 'tap_1010', nickname: '月半弯', avatar: DEMO_AVATAR, platform: 'TapTap', status: 'active', createdAt: '2026-09-12T12:32:00+08:00', lastActiveAt: '2026-09-16T06:16:00+08:00', sessions: 70, totalMatches: 21 },
  { id: 'U1011', username: 'wx_1011', nickname: '小太阳', avatar: DEMO_AVATAR, platform: '微信', status: 'disabled', createdAt: '2026-09-13T10:05:00+08:00', lastActiveAt: '2026-09-15T18:11:00+08:00', sessions: 29, totalMatches: 7 },
  { id: 'U1012', username: 'tap_1012', nickname: '蓝莓汽水', avatar: DEMO_AVATAR, platform: 'TapTap', status: 'active', createdAt: '2026-09-14T15:26:00+08:00', lastActiveAt: '2026-09-16T05:48:00+08:00', sessions: 45, totalMatches: 14 },
  { id: 'U1013', username: 'wx_1013', nickname: '橘子海', avatar: DEMO_AVATAR, platform: '微信', status: 'active', createdAt: '2026-09-15T08:14:00+08:00', lastActiveAt: '2026-09-16T05:09:00+08:00', sessions: 36, totalMatches: 11 },
  { id: 'U1014', username: 'tap_1014', nickname: '风筝线', avatar: DEMO_AVATAR, platform: 'TapTap', status: 'disabled', createdAt: '2026-09-15T11:43:00+08:00', lastActiveAt: '2026-09-15T15:37:00+08:00', sessions: 24, totalMatches: 6 },
  { id: 'U1015', username: 'wx_1015', nickname: '纸飞机', avatar: DEMO_AVATAR, platform: '微信', status: 'active', createdAt: '2026-09-16T07:12:00+08:00', lastActiveAt: '2026-09-16T04:56:00+08:00', sessions: 19, totalMatches: 5 },
  { id: 'U1016', username: 'tap_1016', nickname: '甜筒熊', avatar: DEMO_AVATAR, platform: 'TapTap', status: 'active', createdAt: '2026-09-16T07:46:00+08:00', lastActiveAt: '2026-09-16T04:31:00+08:00', sessions: 17, totalMatches: 4 },
  { id: 'U1017', username: 'wx_1017', nickname: '海盐柠檬', avatar: DEMO_AVATAR, platform: '微信', status: 'disabled', createdAt: '2026-09-16T08:03:00+08:00', lastActiveAt: '2026-09-16T03:54:00+08:00', sessions: 12, totalMatches: 3 },
  { id: 'U1018', username: 'tap_1018', nickname: '夏夜灯', avatar: DEMO_AVATAR, platform: 'TapTap', status: 'active', createdAt: '2026-09-16T08:47:00+08:00', lastActiveAt: '2026-09-16T03:20:00+08:00', sessions: 9, totalMatches: 2 }
];

function getChinaDate(iso) {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return '';
  const chinaDate = new Date(date.getTime() + 8 * 60 * 60 * 1000);
  const month = String(chinaDate.getUTCMonth() + 1).padStart(2, '0');
  const day = String(chinaDate.getUTCDate()).padStart(2, '0');
  return `${chinaDate.getUTCFullYear()}-${month}-${day}`;
}

function getStats(users) {
  const list = Array.isArray(users) ? users : [];
  return {
    total: list.length,
    newToday: list.filter((user) => getChinaDate(user.createdAt) === '2026-09-16').length,
    active: list.filter((user) => user.status === 'active').length,
    disabled: list.filter((user) => user.status === 'disabled').length
  };
}

function formatDate(iso) {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return '--';
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false
  }).format(date);
}

function getInitials(nickname) {
  const text = String(nickname || '').trim();
  if (!text) return '--';
  return text.length <= 2 ? text : `${text.slice(0, 1)}${text.slice(-1)}`;
}

function loadUsers() {
  try {
    const raw = typeof window !== 'undefined' && window.localStorage
      ? window.localStorage.getItem(STORAGE_KEY)
      : null;
    return restoreUserState(raw, SEED_USERS);
  } catch (error) {
    return restoreUserState(null, SEED_USERS);
  }
}

function saveUsers(users) {
  try {
    if (typeof window !== 'undefined' && window.localStorage) {
      window.localStorage.setItem(STORAGE_KEY, serializeUserState(users));
    }
  } catch (error) {
    // Storage is optional for the demo; UI operations should still complete.
  }
}

const state = {
  users: loadUsers(), query: '', status: 'all', platform: 'all',
  page: 1, pageSize: 6, selectedIds: new Set(), detailId: null,
  pendingAction: null
};

function getVisiblePage() {
  const filteredUsers = filterUsers(state.users, {
    query: state.query,
    status: state.status,
    platform: state.platform
  });
  const page = paginateUsers(filteredUsers, state.page, state.pageSize);
  state.page = page.page;
  return { ...page, filteredUsers };
}

function getElement(id) {
  return document.getElementById(id);
}

function createTextElement(tagName, className, text) {
  const element = document.createElement(tagName);
  if (className) element.className = className;
  element.textContent = text;
  return element;
}

function renderStats() {
  const container = getElement('statCards');
  if (!container) return;

  const stats = getStats(state.users);
  const cards = [
    ['用户总数', stats.total, '全部演示账号', ''],
    ['今日新增', stats.newToday, '按中国时区统计', 'stat-card__trend--positive'],
    ['活跃用户', stats.active, '当前正常状态', 'stat-card__trend--positive'],
    ['已禁用用户', stats.disabled, '需要关注的账号', 'stat-card__trend--negative']
  ];
  const fragment = document.createDocumentFragment();
  for (const [label, value, trend, trendClass] of cards) {
    const card = createTextElement('article', 'stat-card', '');
    const heading = createTextElement('h2', '', label);
    const count = createTextElement('strong', '', String(value));
    const note = createTextElement('span', `stat-card__trend ${trendClass}`.trim(), trend);
    card.append(heading, count, note);
    fragment.append(card);
  }
  container.replaceChildren(fragment);
}

function createStatusTag(status) {
  const label = status === 'disabled' ? '已禁用' : '正常';
  const tag = createTextElement('span', `status-tag status-tag--${VALID_STATUSES.has(status) ? status : 'active'}`, label);
  return tag;
}

function createPlatformChip(platform) {
  const platformClass = platform === '微信' ? 'wechat' : platform === 'TapTap' ? 'taptap' : '';
  const className = platformClass ? `platform-chip platform-chip--${platformClass}` : 'platform-chip';
  return createTextElement('span', className, platform);
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
  const avatar = createTextElement('span', 'avatar', '');
  const image = document.createElement('img');
  image.src = user.avatar;
  image.alt = `${user.nickname}头像`;
  image.addEventListener('error', () => {
    avatar.classList.add('avatar--fallback');
    image.remove();
    avatar.textContent = getInitials(user.nickname);
  }, { once: true });
  avatar.append(image);

  const details = createTextElement('div', '', '');
  details.append(
    createTextElement('div', 'user-name', user.nickname),
    createTextElement('div', 'user-meta', user.username)
  );
  wrapper.append(checkbox, avatar, details);
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
    cell.append(
      createTextElement('strong', '', '暂无匹配用户'),
      createTextElement('p', '', '请调整搜索词或筛选条件后重试。')
    );
    row.append(cell);
    fragment.append(row);
  } else {
    for (const user of view.items) {
      const selected = state.selectedIds.has(user.id);
      const row = document.createElement('tr');
      row.dataset.userId = user.id;
      row.className = selected ? 'is-selected' : '';
      row.setAttribute('aria-selected', String(selected));
      row.append(
        createUserCell(user, selected),
        (() => {
          const cell = createTextElement('td', '', '');
          cell.append(createPlatformChip(user.platform));
          return cell;
        })(),
        createTextElement('td', '', formatDate(user.createdAt)),
        createTextElement('td', '', formatDate(user.lastActiveAt)),
        (() => {
          const cell = createTextElement('td', '', '');
          cell.append(createStatusTag(user.status));
          return cell;
        })(),
        (() => {
          const cell = createTextElement('td', '', '');
          const actions = createTextElement('div', 'row-actions', '');
          const button = createTextElement('button', 'row-action', '查看详情');
          button.type = 'button';
          button.dataset.action = 'detail';
          button.dataset.userId = user.id;
          button.setAttribute('aria-label', `查看${user.nickname}详情`);
          actions.append(button);
          cell.append(actions);
          return cell;
        })()
      );
      fragment.append(row);
    }
  }
  body.replaceChildren(fragment);
}

function renderPagination() {
  const container = getElement('pagination');
  if (!container) return;

  const view = getVisiblePage();
  const summary = createTextElement('p', 'pagination__summary',
    `第 ${view.page} / ${view.totalPages} 页，共 ${view.total} 位用户`);
  const controls = createTextElement('div', 'pagination__controls', '');
  const previous = createTextElement('button', '', '上一页');
  previous.type = 'button';
  previous.dataset.action = 'page';
  previous.dataset.page = String(view.page - 1);
  previous.disabled = view.page <= 1;
  previous.setAttribute('aria-label', '上一页');
  const next = createTextElement('button', '', '下一页');
  next.type = 'button';
  next.dataset.action = 'page';
  next.dataset.page = String(view.page + 1);
  next.disabled = view.page >= view.totalPages;
  next.setAttribute('aria-label', '下一页');
  controls.append(previous, next);
  container.replaceChildren(summary, controls);
}

function renderBulkBar() {
  const bar = getElement('batchActions');
  const count = getElement('selectedCount');
  if (!bar || !count) return;

  const existingIds = new Set(state.users.map((user) => user.id));
  state.selectedIds = new Set([...state.selectedIds].filter((id) => existingIds.has(id)));
  const selectedCount = state.selectedIds.size;
  bar.hidden = selectedCount === 0;
  count.textContent = `已选择 ${selectedCount} 位用户`;
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
  const user = state.users.find((item) => item.id === state.detailId);
  if (!user) {
    const wasOpen = state.detailId !== null;
    state.detailId = null;
    drawer.hidden = true;
    drawer.setAttribute('aria-hidden', 'true');
    if (wasOpen) showToast('用户数据已更新，详情已关闭', 'warning');
    return;
  }

  drawer.hidden = false;
  drawer.setAttribute('aria-hidden', 'false');
  getElement('detailUserId').textContent = user.id;
  getElement('detailUsername').textContent = user.username;
  getElement('detailPlatform').textContent = user.platform;
  getElement('detailCreatedAt').textContent = formatDate(user.createdAt);
  getElement('detailLastActiveAt').textContent = formatDate(user.lastActiveAt);
  getElement('detailSessions').textContent = String(user.sessions);
  getElement('detailTotalMatches').textContent = String(user.totalMatches);

  const action = ensureDrawerAction(drawer);
  action.dataset.userId = user.id;
  action.textContent = user.status === 'disabled' ? '启用账号' : '禁用账号';
  action.setAttribute('aria-label', `${action.textContent}${user.nickname}`);
}

function ensureSelectAllCheckbox() {
  const body = getElement('userTableBody');
  const header = body && body.closest('table')
    ? body.closest('table').querySelector('thead th:first-child')
    : null;
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
  const view = getVisiblePage();
  const visibleIds = view.items.map((user) => user.id);
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

function completeStatusChange(ids, status) {
  const existingIds = [...new Set(ids)].filter((id) => state.users.some((user) => user.id === id));
  closeConfirmation();
  if (!existingIds.length) {
    showToast('没有找到可操作的用户', 'warning');
    return;
  }

  state.users = setUserStatus(state.users, existingIds, status);
  saveUsers(state.users);
  state.selectedIds = new Set([...state.selectedIds].filter((id) => !existingIds.includes(id)));
  renderAll();
  showToast(status === 'disabled'
    ? `已禁用 ${existingIds.length} 个账号`
    : `已启用 ${existingIds.length} 个账号`, 'success');
}

function requestStatusChange(ids, status) {
  const existingIds = [...new Set(ids)].filter((id) => state.users.some((user) => user.id === id));
  if (!existingIds.length) {
    showToast('没有选择可操作的用户', 'warning');
    return;
  }
  if (status === 'disabled') {
    state.pendingAction = { ids: existingIds, status };
    openConfirmation(existingIds.length);
    return;
  }
  completeStatusChange(existingIds, status);
}

function confirmPendingAction() {
  if (!state.pendingAction) {
    closeConfirmation();
    return;
  }
  const action = state.pendingAction;
  completeStatusChange(action.ids, action.status);
}

function closeDrawer() {
  state.detailId = null;
  renderDrawer();
}

function openDrawer(id) {
  if (state.users.some((user) => user.id === id)) {
    state.detailId = id;
    renderDrawer();
  }
}

function bindEvents() {
  const search = getElement('searchInput');
  const statusFilter = getElement('statusFilter');
  const platformFilter = getElement('platformFilter');
  const filters = getElement('userFilters');
  const table = getElement('userTableBody') && getElement('userTableBody').closest('table');
  const pagination = getElement('pagination');
  const drawer = getElement('detailDrawer');
  const dialog = getElement('confirmDialog');

  search.addEventListener('input', () => {
    state.query = search.value;
    state.page = 1;
    renderAll();
  });
  statusFilter.addEventListener('change', () => {
    state.status = statusFilter.value;
    state.page = 1;
    renderAll();
  });
  platformFilter.addEventListener('change', () => {
    state.platform = platformFilter.value;
    state.page = 1;
    renderAll();
  });
  filters.addEventListener('submit', (event) => {
    event.preventDefault();
    state.query = search.value;
    state.status = statusFilter.value;
    state.platform = platformFilter.value;
    state.page = 1;
    renderAll();
  });

  table.addEventListener('change', (event) => {
    const target = event.target;
    if (!target || !target.matches('input[type="checkbox"]')) return;
    const view = getVisiblePage();
    if (target.dataset.action === 'select-all') {
      for (const user of view.items) {
        if (target.checked) state.selectedIds.add(user.id);
        else state.selectedIds.delete(user.id);
      }
      renderTable();
      renderBulkBar();
      syncSelectAll();
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
      renderBulkBar();
      syncSelectAll();
    }
  });
  table.addEventListener('click', (event) => {
    const target = event.target;
    const button = target.closest('button[data-action="detail"]');
    if (button) {
      openDrawer(button.dataset.userId);
      return;
    }
    if (target.closest('input, button, a')) return;
    const row = target.closest('tr[data-user-id]');
    if (row) openDrawer(row.dataset.userId);
  });

  pagination.addEventListener('click', (event) => {
    const button = event.target.closest('button[data-action="page"]');
    if (!button || button.disabled) return;
    state.page = Number(button.dataset.page) || 1;
    renderAll();
  });

  getElement('refreshButton').addEventListener('click', () => {
    state.users = loadUsers();
    state.selectedIds.clear();
    state.page = 1;
    renderAll();
    showToast('用户数据已刷新', 'success');
  });
  getElement('batchDisableButton').addEventListener('click', () => {
    if (!state.selectedIds.size) {
      showToast('请先选择要禁用的用户', 'warning');
      return;
    }
    requestStatusChange([...state.selectedIds], 'disabled');
  });
  getElement('closeDetailButton').addEventListener('click', closeDrawer);
  drawer.addEventListener('click', (event) => {
    const button = event.target.closest('button[data-action="detail-status"]');
    if (!button) return;
    const user = state.users.find((item) => item.id === button.dataset.userId);
    if (user) requestStatusChange([user.id], user.status === 'disabled' ? 'active' : 'disabled');
  });

  const dialogForm = dialog.querySelector('form');
  dialogForm.addEventListener('submit', (event) => {
    event.preventDefault();
    if (event.submitter && event.submitter.value === 'confirm') confirmPendingAction();
    else closeConfirmation();
  });
  dialog.addEventListener('cancel', (event) => {
    event.preventDefault();
    closeConfirmation();
  });
  dialog.addEventListener('click', (event) => {
    if (event.target === dialog) closeConfirmation();
  });
  document.addEventListener('keydown', (event) => {
    if (event.key !== 'Escape') return;
    if (isDialogOpen(dialog)) {
      event.preventDefault();
      closeConfirmation();
    } else if (state.detailId !== null) {
      event.preventDefault();
      closeDrawer();
    }
  });
}

function boot() {
  if (document.documentElement.dataset.adminBooted === 'true') return;
  document.documentElement.dataset.adminBooted = 'true';
  const dialog = getElement('confirmDialog');
  if (dialog && typeof dialog.showModal !== 'function') dialog.hidden = true;
  ensureSelectAllCheckbox();
  bindEvents();
  renderAll();
}

const AdminCore = {
  STORAGE_KEY,
  SEED_USERS,
  filterUsers,
  paginateUsers,
  setUserStatus,
  serializeUserState,
  restoreUserState,
  getStats,
  formatDate,
  getInitials
};
if (typeof module !== 'undefined' && module.exports) module.exports = AdminCore;
if (typeof window !== 'undefined') window.AdminCore = AdminCore;

if (typeof document !== 'undefined') {
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', boot, { once: true });
  } else {
    boot();
  }
}
