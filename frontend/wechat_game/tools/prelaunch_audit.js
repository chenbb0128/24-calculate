/*
 * 上线前本地审计：不需要 wx，不会修改存档或项目配置。
 * 运行：node tools/prelaunch_audit.js
 */
const assert = require('assert');
const fs = require('fs');
const path = require('path');

const ROOT = path.resolve(__dirname, '..');
const SRC = path.join(ROOT, 'src');
const warnings = [];
const strictRelease = process.argv.includes('--strict') || process.env.RELEASE_STRICT === '1';

function check(condition, message) {
  assert.ok(condition, message);
}

function readJson(file) {
  return JSON.parse(fs.readFileSync(path.join(ROOT, file), 'utf8'));
}

function listFiles(directory, suffix) {
  const result = [];
  fs.readdirSync(directory, { withFileTypes: true }).forEach((entry) => {
    const full = path.join(directory, entry.name);
    if (entry.isDirectory()) result.push(...listFiles(full, suffix));
    else if (!suffix || full.endsWith(suffix)) result.push(full);
  });
  return result;
}

function testProjectConfig() {
  const project = readJson('project.config.json');
  // project.private.config.json is a local WeChat DevTools override and is
  // intentionally ignored by Git; a clean checkout must still be auditable.
  const privateConfig = fs.existsSync(path.join(ROOT, 'project.private.config.json'))
    ? readJson('project.private.config.json')
    : project;
  const game = readJson('game.json');
  const apiClient = require('../src/services/api_client.js');
  check(project.appid === 'wx1e7ac815548c561c', '正式 AppID 与 project.config.json 不一致');
  check(privateConfig.appid === project.appid, '两个项目配置的 AppID 不一致');
  check(project.compileType === 'game' && privateConfig.compileType === 'game', '项目不是微信小游戏类型');
  check(game.deviceOrientation === 'portrait', '小游戏必须保持竖屏配置');
  check(fs.existsSync(path.join(ROOT, 'game.js')), '缺少游戏入口 game.js');
  check(fs.existsSync(path.join(SRC, 'app.js')), '缺少前端主逻辑 src/app.js');
  check(apiClient.API_BASE_URL === 'https://calc-api.pdurl.cn', '正式 API 地址与线上域名不一致');
  check(apiClient.isConfigured(), '正式 API 地址未启用');
  ['project', 'privateConfig'].forEach((name) => {
    const config = name === 'project' ? project : privateConfig;
    const setting = config.setting || {};
    check(setting.urlCheck === true, `${name} 必须开启合法域名校验`);
    check(setting.minified === true, `${name} 必须开启代码压缩`);
  });
  check(project.setting.uploadWithSourceMap === false, '正式包不应上传源码映射');
  const apiSource = fs.readFileSync(path.join(SRC, 'services', 'api_client.js'), 'utf8');
  check(/const\s+ALLOW_LOCAL_BACKEND\s*=\s*false/.test(apiSource), '正式包不能开启本地后端兜底');
  const platformSource = fs.readFileSync(path.join(SRC, 'services', 'platform.js'), 'utf8');
  if (platformSource.includes('adunit-placeholder')) warnings.push('广告位仍是占位符，正式发布前必须替换真实广告位 ID');
}

function testAssets() {
  const audioDirectory = path.join(ROOT, 'assets', 'audio');
  const expected = [
    'music_home_childlike.wav', 'music_level_childlike.wav',
    'click.wav', 'card.wav', 'operator.wav', 'merge.wav', 'success.wav', 'error.wav',
    'countdown_tick.wav', 'countdown_urgent.wav',
  ];
  expected.forEach((name) => {
    const file = path.join(audioDirectory, name);
    check(fs.existsSync(file), `缺少音频资源 ${name}`);
    check(fs.statSync(file).size > 32, `音频资源为空 ${name}`);
  });
}

function testStorageAndLeaderboard() {
  const storage = require('../src/services/storage.js');
  const leaderboard = require('../src/services/leaderboard_service.js');
  const progress = storage.normalize({
    coins: Infinity,
    unlocked_level: 9999,
    levels: { '0': { stars: 99, best_score: Infinity }, '1': { stars: 2, best_score: 80 } },
    leaderboards: { endless: { best_score: Infinity, last_score: 'bad' } },
  });
  check(progress.coins === 0, '异常金币没有安全归零');
  check(progress.unlocked_level === 200, '关卡进度没有安全限制');
  check(progress.levels['0'].stars === 3 && progress.levels['0'].best_score === 0, '异常关卡记录没有安全归一化');
  check(Number.isFinite(leaderboard.playerScore(progress, leaderboard.MODE_CAMPAIGN)), '本地排行榜分数不是有限数字');

  const remote = {
    my_user_id: 'not-a-number',
    my_rank: '6',
    my_score: '240',
    entries: [
      { user_id: 'bad', nickname: '异常数据', score: Infinity, rank: 'bad' },
      { user_id: 2, nickname: '玩家2', score: 120, rank: 2 },
    ],
  };
  const entries = leaderboard.getRemoteEntries(remote, 0);
  check(entries.length === 2 && entries.every((entry) => Number.isFinite(entry.score) && Number.isFinite(entry.rank)), '排行榜异常数据未被清洗');
  const mine = leaderboard.personalSummary(progress, leaderboard.MODE_ENDLESS, remote, 0);
  check(mine.score === 240 && mine.rank === 6 && mine.source === 'server', '排行榜个人成绩没有优先使用服务端数据');
}

function testServerAuthoritativeSettlement() {
  const appSource = fs.readFileSync(path.join(SRC, 'app', 'game_app.js'), 'utf8');
  const settlementSource = appSource.slice(appSource.indexOf('const localProgressAllowed'));
  check(/if\s*\(localProgressAllowed\)\s*storage\.saveLevel\s*\(/.test(settlementSource), '正式闯关结算必须只在本地模式保存解锁');
}

function testCoreContracts() {
  const puzzle = require('../src/core/puzzle_generator.js');
  const levels = require('../src/core/level_catalog.js').all();
  const daily = require('../src/core/daily_challenge.js');
  const question = puzzle.makeVerifiedRecord([1, 2, 4, 5], 900001, 0);
  check(question && puzzle.isVerifiedRecord(question), '核心题目验证失败');
  check(levels.length === 200, '闯关关卡数量不是 200');
  const dailyResult = daily.build(puzzle, '2026-08-16', 20260816);
  check(dailyResult.puzzles.length === daily.DAILY_QUESTION_COUNT, '每日挑战题目数量不完整');
  check(dailyResult.puzzles.every((record) => puzzle.isVerifiedRecord(record)), '每日挑战存在未验证题目');
}

function testAppGuards() {
  const { GameApp } = require('../src/app.js');
  const app = Object.create(GameApp.prototype);
  Object.assign(app, { score: 60, mistakes: 0, timeLeft: 30, timerLimit: 60 });
  check(app.calculateCampaignStars().stars === 1, '60 分星级规则错误');
  app.score = 80;
  check(app.calculateCampaignStars().stars === 2, '80 分星级规则错误');
  app.score = 100;
  check(app.calculateCampaignStars().stars === 3, '100 分星级规则错误');
  let finished = false;
  Object.assign(app, {
    screen: 'game', transitioning: false, settling: false, settleToken: 0, cards: [{ value: 24 }], currentPuzzle: { rules: {} },
    questionOperators: ['+'], finish() { finished = true; },
  });
  check(app.checkSolvedState() === true, '最终得到 24 没有触发结算检查');
  return new Promise((resolve) => setTimeout(() => {
    check(finished, '最终得到 24 没有进入结算流程');
    resolve();
  }, 0));
}

function testSourceSyntax() {
  const files = listFiles(SRC, '.js');
  files.forEach((file) => {
    const source = fs.readFileSync(file, 'utf8');
    check(!/\bTODO\s*:/i.test(source), `源码仍包含未处理 TODO：${path.relative(ROOT, file)}`);
  });
}

async function main() {
  testProjectConfig();
  testAssets();
  testStorageAndLeaderboard();
  testServerAuthoritativeSettlement();
  testCoreContracts();
  await testAppGuards();
  testSourceSyntax();
  if (strictRelease && warnings.length) {
    throw new Error(`PRELAUNCH_AUDIT_BLOCKED warnings=${warnings.length}: ${warnings.join('; ')}`);
  }
  console.log(`PRELAUNCH_AUDIT_OK warnings=${warnings.length}`);
  warnings.forEach((warning) => console.log(`WARNING ${warning}`));
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
