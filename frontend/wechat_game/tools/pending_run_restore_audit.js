/* Verify startup-style Run recovery and terminal checkpoint cleanup. */
const assert = require('assert');

const memory = Object.create(null);
global.wx = {
  getStorageSync(key) { return memory[key]; },
  setStorageSync(key, value) { memory[key] = value; },
  removeStorageSync(key) { delete memory[key]; },
  request(options) {
    const path = String(options.url || '').replace(/^https:\/\/calc-api\.pdurl\.cn/, '');
    const success = (statusCode, data) => options.success({ statusCode, data: JSON.stringify(data) });
    if (path.endsWith('/campaign/runs/campaign-active')) {
      success(200, {
        code: 0,
        message: 'success',
        data: {
          run_id: 'campaign-active', status: 'running', level_id: 2, question_index: 1, score: 35, mistakes: 1, hints_used: 0, best_combo: 2,
          puzzles: [{ puzzle_id: 'c1', numbers: [1, 2, 4, 5] }, { puzzle_id: 'c2', numbers: [3, 3, 4, 6] }],
          attempts: [{ puzzle_id: 'c1', question_index: 0, solved: true, score: 35 }],
        },
      });
      return;
    }
    if (path.endsWith('/daily/runs/daily-finished')) {
      success(200, { code: 0, message: 'success', data: { run_id: 'daily-finished', status: 'finished', completed: true } });
      return;
    }
    if (path.endsWith('/daily/runs/daily-active')) {
      success(200, {
        code: 0,
        message: 'success',
        data: {
          run_id: 'daily-active', status: 'running', date_key: '2026-08-18', question_index: 1,
          score: 40, mistakes: 0, hints_used: 0, best_combo: 1,
          puzzles: [{ puzzle_id: 'd1', numbers: [1, 2, 4, 5] }, { puzzle_id: 'd2', numbers: [3, 3, 4, 6] }],
          attempts: [{ puzzle_id: 'd1', question_index: 0, solved: true, score: 40 }],
        },
      });
      return;
    }
    if (path.endsWith('/endless/runs/endless-expired')) {
      success(404, { code: 40404, message: 'run expired', data: null });
      return;
    }
    if (path.endsWith('/endless/runs/endless-active')) {
      success(200, {
        code: 0,
        message: 'success',
        data: {
          run_id: 'endless-active', status: 'running', question_index: 2,
          score: 60, mistakes: 0, hints_used: 0, best_combo: 2,
          puzzles: [
            { puzzle_id: 'e1', numbers: [1, 2, 4, 5] },
            { puzzle_id: 'e2', numbers: [3, 3, 4, 6] },
            { puzzle_id: 'e3', numbers: [2, 3, 4, 6] },
          ],
          attempts: [
            { puzzle_id: 'e1', question_index: 0, solved: true, score: 30 },
            { puzzle_id: 'e2', question_index: 1, solved: true, score: 30 },
          ],
        },
      });
      return;
    }
    options.fail({ errMsg: `unexpected request ${path}` });
  },
};
memory.twenty_four_auth = { access_token: 'access', refresh_token: 'refresh', token_type: 'Bearer' };

const storage = require('../src/services/storage.js');
const { GameApp } = require('../src/app/game_app.js');

function check(condition, message) { assert.ok(condition, message); }

async function run() {
  storage.setAccount('restore-account');
  const progress = storage.load();
  storage.savePendingRun({ mode: 'campaign', run_id: 'campaign-active', level_id: 2, question_index: 0, attempts: [] }, progress);
  storage.savePendingRun({ mode: 'daily', run_id: 'daily-finished', date_key: '2026-08-18', attempts: [] });
  storage.savePendingRun({ mode: 'endless', run_id: 'endless-expired', question_index: 3, attempts: [{ question_index: 0 }] });

  const app = Object.create(GameApp.prototype);
  app.backendAuth = { status: 'ready' };
  app.friendRoomFromInvite = false;
  app.mode = 'campaign';
  app.screen = 'home';
  app.progress = storage.load();
  app.feedback = {};
  app.messages = [];
  app.triggerFeedback = (type, text) => app.messages.push({ type, text });
  app.applyResumedRun = (record) => { app.resumed = record; return true; };

  await app.restorePendingRuns();
  check(!app.resumed, '启动时不应自动打开 active campaign Run');
  check(app.pendingResumeRuns && app.pendingResumeRuns.campaign && app.pendingResumeRuns.campaign.status === 'active', 'active campaign Run 未保存为待恢复记录');
  check(!storage.getPendingRun('daily'), 'finished Run 没有清理 pending');
  check(!storage.getPendingRun('endless'), 'expired Run 没有清理 pending');
  check(storage.getPendingRun('campaign').run_id === 'campaign-active', 'active Run 被错误清理');
  storage.clearPendingRun('campaign', 'campaign-active', app.progress);
  storage.savePendingRun({ mode: 'daily', run_id: 'daily-active', date_key: '2026-08-18', attempts: [{ question_index: 0 }] }, app.progress);
  app.resumed = null;
  app.screen = 'home';
  app.mode = 'campaign';
  await app.restorePendingRuns();
  check(!app.resumed, 'active daily Run should not auto-open on startup');
  check(app.screen === 'home', 'startup must remain on home when an active daily Run exists');
  check(app.pendingResumeRuns && app.pendingResumeRuns.daily && app.pendingResumeRuns.daily.status === 'active', 'active daily Run was not kept for explicit resume');
  app.startDaily();
  check(app.resumed && app.resumed.pending.mode === 'daily', 'Daily Challenge tap did not resume the active daily Run');

  storage.clearPendingRun('daily', 'daily-active', app.progress);
  storage.savePendingRun({ mode: 'endless', run_id: 'endless-active', question_index: 2, attempts: [{ question_index: 0 }, { question_index: 1 }] }, app.progress);
  app.resumed = null;
  app.screen = 'home';
  app.mode = 'campaign';
  await app.restorePendingRuns();
  check(!app.resumed, 'active endless Run should not auto-open on startup');
  check(app.pendingResumeRuns && app.pendingResumeRuns.endless && app.pendingResumeRuns.endless.status === 'active', 'active endless Run was not kept for explicit resume');
  app.startEndless();
  check(app.resumed && app.resumed.pending.mode === 'endless', 'Endless tap did not resume the active endless Run');
  console.log('PENDING_RUN_RESTORE_OK');
}

run().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
