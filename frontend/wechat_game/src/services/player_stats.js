const DEFAULT_STATS = { total_solved: 0, total_score: 0, fastest_ms: 0, best_combo: 0, best_level: 0, best_chapter: 0, operator_counts: {}, mode_questions: {}, last_solve: {} };
function ensureProgress(progress) { if (!progress.player_stats || typeof progress.player_stats !== 'object') progress.player_stats = JSON.parse(JSON.stringify(DEFAULT_STATS)); Object.keys(DEFAULT_STATS).forEach((key) => { if (progress.player_stats[key] === undefined) progress.player_stats[key] = typeof DEFAULT_STATS[key] === 'object' ? {} : DEFAULT_STATS[key]; }); }
function recordSolve(progress, modeId, elapsedMs, score, combo, operators, levelIndex = -1) {
  ensureProgress(progress); const stats = progress.player_stats; stats.total_solved += 1; stats.total_score += Math.max(0, Number(score || 0)); stats.best_combo = Math.max(stats.best_combo, Number(combo || 0));
  if (Number(elapsedMs) > 0 && (!stats.fastest_ms || elapsedMs < stats.fastest_ms)) stats.fastest_ms = Number(elapsedMs);
  if (levelIndex >= 0) { stats.best_level = Math.max(stats.best_level, Number(levelIndex) + 1); stats.best_chapter = Math.max(stats.best_chapter, Math.floor(Number(levelIndex) / 20) + 1); }
  (operators || []).forEach((operator) => { stats.operator_counts[operator] = Number(stats.operator_counts[operator] || 0) + 1; });
  stats.mode_questions[modeId] = Number(stats.mode_questions[modeId] || 0) + 1; stats.last_solve = { mode: modeId, elapsed_ms: elapsedMs, score };
}
function summary(progress) { ensureProgress(progress); const stats = progress.player_stats; const entries = Object.entries(stats.operator_counts || {}).sort((a, b) => b[1] - a[1]); return { ...stats, favorite_operator: entries.length ? entries[0][0] : '暂无', favorite_count: entries.length ? entries[0][1] : 0, mode_questions: { ...stats.mode_questions } }; }
module.exports = { DEFAULT_STATS, ensureProgress, recordSolve, summary };
