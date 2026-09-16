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

function serializeUserState(users) {
  return JSON.stringify(users.map((user) => ({ id: user.id, status: user.status })));
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
    const status = savedStatus === 'active' || savedStatus === 'disabled'
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

const AdminCore = {
  STORAGE_KEY,
  SEED_USERS,
  filterUsers,
  paginateUsers,
  setUserStatus,
  serializeUserState,
  restoreUserState
};
if (typeof module !== 'undefined' && module.exports) module.exports = AdminCore;
if (typeof window !== 'undefined') window.AdminCore = AdminCore;
