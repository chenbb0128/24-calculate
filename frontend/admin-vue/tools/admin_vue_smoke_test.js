const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const root = path.join(__dirname, '..');
const read = (file) => fs.readFileSync(path.join(root, file), 'utf8');

const router = read('src/router/index.js');
const userStore = read('src/store/modules/user.js');
const auth = read('src/utils/auth.js');
const userView = read('src/views/users/index.vue');
const adminApi = read('src/api/admin.js');
const loginView = read('src/views/login/index.vue');
const vueConfig = read('vue.config.js');
const main = read('src/main.js');

assert.match(router, /redirect:\s*['"]\/users['"]/);
assert.match(router, /path:\s*['"]users['"]/);
assert.match(router, /views\/users\/index/);
assert.doesNotMatch(router, /path:\s*['"]\/example['"]/);
assert.match(userStore, /adminLogin|login\(/);
assert.match(userStore, /access_token/);
assert.match(auth, /sessionStorage/);
assert.doesNotMatch(auth, /js-cookie|Cookies/);
assert.match(userView, /用户管理/);
assert.match(userView, /禁用用户|启用用户/);
assert.match(adminApi, /\/auth\/login/);
assert.match(adminApi, /\/users/);
assert.match(adminApi, /sessionStorage/);
assert.match(adminApi, /NICKNAME_REJECTED|AVATAR_REJECTED|MODERATION_PENDING|NICKNAME_EDIT_DISABLED/);
assert.doesNotMatch(adminApi, /localStorage/);
assert.match(loginView, /password:\s*['"]['"]/);
assert.doesNotMatch(loginView, /password:\s*['"]111111['"]/);
assert.match(loginView, /\$message\.error/);
assert.match(vueConfig, /['"]\/api['"]\s*:/);
assert.match(vueConfig, /target:\s*['"]http:\/\/127\.0\.0\.1:8080['"]/);
assert.doesNotMatch(main, /mockXHR/);

console.log('PASS: vue admin dashboard smoke test');
