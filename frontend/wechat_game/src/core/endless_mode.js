function configForQuestion(questionIndex, performance = {}) {
  const baseStage = Math.max(0, Math.floor(Number(questionIndex) / 3));
  const speedRatio = Number(performance.speed_ratio || performance.speedRatio || 1);
  const fastStreak = Number(performance.fast_streak || performance.fastStreak || 0);
  const aiStageBonus = fastStreak >= 2 && speedRatio <= 0.62 ? 1 : 0;
  const aiStage = baseStage + aiStageBonus;
  let timeLimit = Math.max(18, 45 - baseStage);
  if (aiStageBonus > 0) timeLimit = Math.max(18, timeLimit - 2);
  return {
    stage: baseStage,
    ai_stage: aiStage,
    stage_name: stageName(aiStage),
    question_index: Number(questionIndex),
    question_count: 1,
    time_limit: timeLimit,
    min_digit: 1,
    max_digit: aiStage >= 4 ? 13 : 9,
    min_solutions: aiStage < 2 ? 2 : 1,
    // Keep a small candidate pool in late stages, but never collapse it to
    // 1-2 solutions: that made random generation occasionally return no
    // question and broke the promise of an endless run.
    max_solutions: aiStage < 2 ? 999999 : Math.max(4, 12 - Math.floor(aiStage / 2)),
    // Solution count alone is not enough to express difficulty. Add a soft
    // score floor so a later stage cannot randomly fall back to a beginner
    // puzzle just because it found an easy valid combination first.
    // Keep a real difficulty floor, not only a solution-count restriction.
    // The previous floor allowed a later stage to fall back to a puzzle that
    // was easier than the one before it when the random candidate order
    // changed. The stepped floor makes the endless curve visibly progress
    // while still leaving room for the generator's fallback path.
    // The available integer 1..13 space has a practical solver ceiling.
    // After reaching it, endless mode keeps pressure through time, solution
    // count and hint limits instead of exhausting the candidate pool.
    min_difficulty: Math.min(12, aiStage === 5 ? 11 : 4 + Math.floor(aiStage * 1.2)),
    allow_hint: aiStage < 5,
    hint_count: aiStage < 3 ? 1 : 0,
    generator: 'local_ai_director',
  };
}

function scoreMultiplier(combo, fastStreak) { return Math.min(3, 1 + Math.floor(Number(combo) / 5) * 0.25 + (Number(fastStreak) >= 2 ? 0.25 : 0)); }

function milestoneForQuestions(questions) {
  if (questions <= 0 || questions % 5 !== 0) return {};
  return { questions, reward: 20 + Math.floor(questions / 5) * 5, title: `里程碑达成：连续答对 ${questions} 题` };
}

function nextMilestoneForQuestions(questions) {
  const current = Math.max(0, Math.floor(Number(questions) || 0));
  const milestones = [5, 10, 20, 30, 50, 75, 100, 150, 200];
  const found = milestones.find((milestone) => milestone > current);
  if (found) return found;
  return Math.ceil((current + 1) / 50) * 50;
}

function statusForQuestion(questionIndex) {
  const question = Math.max(0, Math.floor(Number(questionIndex) || 0));
  const config = configForQuestion(question);
  return {
    ...config,
    question_in_stage: question % 3 + 1,
    stage_question_count: 3,
    next_milestone: nextMilestoneForQuestions(question + 1),
  };
}

function seeded(seed) {
  let state = (Number(seed) >>> 0) || 1;
  return { next() { state = (1664525 * state + 1013904223) >>> 0; return state / 0x100000000; }, int(min, max) { return Math.floor(this.next() * (max - min + 1)) + min; } };
}

function numbersKey(numbers) { return numbers.slice().sort((a, b) => a - b).join(','); }

function generateQuestion(generator, questionIndex, runSeed, usedKeys = {}, performance = {}) {
  const config = configForQuestion(questionIndex, performance);
  const rng = seeded(Math.abs(Number(runSeed) + Number(questionIndex) * 7919 + Number(questionIndex) ** 2 * 17) + 1);
  const attempts = 420;
  function tryGenerate(minSolutions, maxSolutions, maxAttempts) {
    for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
      const numbers = [0, 0, 0, 0].map(() => rng.int(config.min_digit, config.max_digit));
      if (config.max_digit >= 13 && !numbers.some((value) => value >= 10)) numbers[rng.int(0, 3)] = rng.int(10, 13);
      const key = numbersKey(numbers);
      if (usedKeys[key]) continue;
      const record = generator.makeVerifiedRecord(numbers, 2000 + Number(questionIndex), Number(questionIndex), minSolutions, maxSolutions);
      if (!record || !record.numbers || Number(record.difficultyScore || 0) < Number(config.min_difficulty || 0)) continue;
      usedKeys[key] = true;
      record.puzzle_id = `ENDLESS-Q${String(Number(questionIndex) + 1).padStart(4, '0')}`;
      record.endless_stage = config.stage;
      record.endless_ai_stage = config.ai_stage;
      record.endless_stage_name = config.stage_name;
      record.generation = { source: 'local_ai_director', validated: true, validator: 'PuzzleGenerator.solve', candidate_attempts: attempt, number_key: key };
      return record;
    }
    return null;
  }
  return tryGenerate(config.min_solutions, config.max_solutions, attempts) || tryGenerate(1, 999999, 240);
}

function stageName(stage) {
  return ['热身', '加速', '进阶', '困难', '高压'][stage] || `极限 ${stage - 4}`;
}

module.exports = { configForQuestion, scoreMultiplier, milestoneForQuestions, nextMilestoneForQuestions, statusForQuestion, generateQuestion, stageName };
