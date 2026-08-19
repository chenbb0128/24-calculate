/*
 * Shared question service for every game mode.
 *
 * This module is deliberately independent from wx and Canvas. It can be used
 * by the game client now and by a server-side validator later. Every returned
 * puzzle has gone through the same solver/validator before it is exposed.
 */
const puzzleGenerator = require('./puzzle_generator.js');
const dailyChallenge = require('./daily_challenge.js');
const endlessMode = require('./endless_mode.js');
const friendMatch = require('./friend_match_service.js');
const levelCatalog = require('./level_catalog.js');
const campaignPuzzleData = require('./campaign_puzzle_data.js');

const RESERVE_NUMBERS = [
  [1, 2, 4, 5], [1, 1, 2, 6], [3, 3, 4, 6], [2, 3, 4, 6],
  [1, 5, 5, 5], [2, 4, 7, 8], [3, 5, 7, 9], [4, 4, 6, 8],
];

function clone(value) {
  if (value === undefined || value === null) return value;
  return JSON.parse(JSON.stringify(value));
}

function safeInteger(value, fallback = 0) {
  const number = Number(value);
  return Number.isFinite(number) ? Math.floor(number) : fallback;
}

function normalizeSeed(value, fallback = 1) {
  const number = safeInteger(value, 0);
  return (Math.abs(number) >>> 0) || fallback;
}

function hashSeed(...parts) {
  let state = 2166136261;
  String(parts.join('|')).split('').forEach((character) => {
    state ^= character.charCodeAt(0);
    state = Math.imul(state, 16777619);
  });
  return (state >>> 0) || 1;
}

function normalizeMode(mode) {
  return String(mode || '').trim().toLowerCase();
}

function ruleValue(config, camel, snake, fallback = '') {
  if (!config) return fallback;
  if (config[camel] !== undefined) return config[camel];
  if (config[snake] !== undefined) return config[snake];
  return fallback;
}

class QuestionService {
  constructor(options = {}) {
    this.generator = options.generator || puzzleGenerator;
    this.levels = Array.isArray(options.levels) ? options.levels : levelCatalog.all();
    this.campaignData = options.campaignData || campaignPuzzleData;
    this.campaignSeedBase = normalizeSeed(options.campaignSeedBase, 240000);
    this.campaignBank = options.campaignBank || null;
    this.campaignBankReady = Boolean(options.campaignBank);
    this.cache = Object.create(null);
    this.usedKeys = Object.create(null);
    this.lastError = '';
  }

  key(numbers) {
    if (this.generator && typeof this.generator.numberKey === 'function') {
      return this.generator.numberKey(numbers);
    }
    return (Array.isArray(numbers) ? numbers : []).slice().sort((a, b) => a - b).join(',');
  }

  seedFor(mode, seed, index = 0, extra = '') {
    return hashSeed(normalizeMode(mode), normalizeSeed(seed), safeInteger(index), extra);
  }

  scopeSet(scope) {
    const name = String(scope || 'default');
    if (!this.usedKeys[name]) this.usedKeys[name] = new Set();
    return this.usedKeys[name];
  }

  clearScope(scope) {
    delete this.usedKeys[String(scope || 'default')];
  }

  isVerified(record, rules = null) {
    if (!record || !Array.isArray(record.numbers) || record.numbers.length !== 4) return false;
    if (!Array.isArray(record.solutionSteps) || record.solutionSteps.length !== 3) return false;
    if (!record.solution || Number(record.target || 24) !== 24) return false;
    if (rules) {
      const minDigit = Number(ruleValue(rules, 'minDigit', 'min_digit', NaN));
      const maxDigit = Number(ruleValue(rules, 'maxDigit', 'max_digit', NaN));
      if (Number.isFinite(minDigit) && record.numbers.some((value) => Number(value) < minDigit)) return false;
      if (Number.isFinite(maxDigit) && record.numbers.some((value) => Number(value) > maxDigit)) return false;
    }
    if (!this.generator || typeof this.generator.isVerifiedRecord !== 'function') return false;
    if (!this.generator.isVerifiedRecord(record)) return false;
    if (!rules || typeof this.generator.executeSteps !== 'function') return true;
    const mergedRules = Object.assign({}, record.rules || {}, rules);
    return Boolean(this.generator.executeSteps(record.numbers, record.solutionSteps, mergedRules));
  }

  remember(scope, record) {
    if (!this.isVerified(record)) return false;
    const set = this.scopeSet(scope);
    const key = this.key(record.numbers);
    if (set.has(key)) return false;
    set.add(key);
    return true;
  }

  withGenerationMeta(record, source, seed, scope) {
    const result = clone(record);
    result.generation = Object.assign({}, result.generation || {}, {
      source,
      validated: true,
      validator: 'QuestionService + PuzzleGenerator.solve',
      seed: normalizeSeed(seed),
      scope: String(scope || ''),
      number_key: this.key(result.numbers),
    });
    return result;
  }

  ensureCampaignBank() {
    if (this.campaignBankReady) return this.campaignBank;
    this.campaignBankReady = true;
    if (!this.generator || typeof this.generator.loadCampaignPuzzleBankFromData !== 'function') return null;
    const loaded = this.generator.loadCampaignPuzzleBankFromData(
      this.campaignData,
      this.levels,
      this.campaignSeedBase,
    );
    this.campaignBank = loaded ? loaded.bank : null;
    return this.campaignBank;
  }

  reserveRecord(config = {}, levelIndex = 0, questionIndex = 0, seed = 1) {
    const requiredOperator = ruleValue(config, 'requiredOperator', 'required_operator', '');
    const forbiddenOperator = ruleValue(config, 'forbiddenOperator', 'forbidden_operator', '');
    const allowNegativeIntermediate = ruleValue(config, 'allowNegativeIntermediate', 'allow_negative_intermediate', false);
    const minSolutions = Number(ruleValue(config, 'minSolutions', 'min_solutions', 1));
    const maxSolutions = Number(ruleValue(config, 'maxSolutions', 'max_solutions', 999999));
    for (let index = 0; index < RESERVE_NUMBERS.length; index += 1) {
      const numbers = RESERVE_NUMBERS[(normalizeSeed(seed) + index) % RESERVE_NUMBERS.length];
      const record = this.generator.makeVerifiedRecord(
        numbers,
        levelIndex,
        questionIndex,
        minSolutions,
        maxSolutions,
        requiredOperator,
        forbiddenOperator,
        allowNegativeIntermediate,
      );
      if (this.isVerified(record, config)) return record;
    }
    return null;
  }

  getCampaignQuestion(levelIndex, questionIndex, options = {}) {
    const level = safeInteger(levelIndex, 0);
    const question = safeInteger(questionIndex, 0);
    const cacheKey = `campaign:${level}:${question}`;
    if (this.cache[cacheKey]) return clone(this.cache[cacheKey]);
    const config = options.config || this.levels[level] || {};
    const scope = 'campaign';
    const bank = this.ensureCampaignBank();
    const staticRecord = bank && bank[level] && bank[level][question];
    if (staticRecord && this.isVerified(staticRecord, config)) {
      const result = this.withGenerationMeta(staticRecord, 'static_campaign_bank', this.campaignSeedBase, scope);
      this.remember(scope, result);
      this.cache[cacheKey] = result;
      return clone(result);
    }

    const seed = this.seedFor('campaign', this.campaignSeedBase, level * 9973 + question);
    const used = this.scopeSet(scope);
    let generated = [];
    if (this.generator && typeof this.generator.generatePuzzleSet === 'function') {
      generated = this.generator.generatePuzzleSet(config, level, 1, seed, used);
    }
    let result = generated && generated[0] && this.isVerified(generated[0], config)
      ? this.withGenerationMeta(generated[0], 'seeded_campaign_generation', seed, scope)
      : null;
    if (!result) {
      const reserve = this.reserveRecord(config, level, question, seed);
      if (reserve && !used.has(this.key(reserve.numbers))) {
        result = this.withGenerationMeta(reserve, 'verified_reserve', seed, scope);
      }
    }
    if (!result) {
      this.lastError = `campaign question unavailable: ${level}:${question}`;
      return null;
    }
    used.add(this.key(result.numbers));
    this.cache[cacheKey] = result;
    return clone(result);
  }

  getCampaignLevel(levelIndex, options = {}) {
    const level = safeInteger(levelIndex, 0);
    const config = options.config || this.levels[level] || {};
    const count = Math.max(1, Number(config.questionCount || config.question_count || options.count || 3));
    const result = [];
    for (let index = 0; index < count; index += 1) {
      const record = this.getCampaignQuestion(level, index, { config });
      if (!record) return [];
      result.push(record);
    }
    return result;
  }

  validDailyResult(result) {
    if (!result || !Array.isArray(result.puzzles) || result.puzzles.length !== dailyChallenge.DAILY_QUESTION_COUNT) return false;
    const keys = new Set();
    return result.puzzles.every((record) => {
      const key = this.key(record.numbers);
      if (keys.has(key) || !this.isVerified(record)) return false;
      keys.add(key);
      return true;
    });
  }

  buildDailyFallback(dateKey, dateSeed) {
    const ruleIndex = ((safeInteger(dateSeed, 0) % dailyChallenge.RULE_COUNT) + dailyChallenge.RULE_COUNT) % dailyChallenge.RULE_COUNT;
    const rule = dailyChallenge.ruleForIndex(ruleIndex);
    const puzzles = [];
    for (let stage = 0; stage < dailyChallenge.DAILY_QUESTION_COUNT; stage += 1) {
      const record = this.reserveRecord({
        requiredOperator: rule.requiredOperator || '',
        forbiddenOperator: rule.forbiddenOperator || '',
        allowNegativeIntermediate: false,
      }, 3000 + stage, stage, this.seedFor('daily-fallback', dateSeed, stage));
      if (!record) return null;
      record.daily_stage = stage;
      record.daily_stage_name = dailyChallenge.DAILY_STAGE_NAMES[stage];
      record.daily_rule_id = rule.id;
      puzzles.push(record);
    }
    return {
      date_key: dateKey,
      seed: normalizeSeed(dateSeed),
      title: '每日挑战 · 五题连战',
      rule_id: rule.id,
      rule_title: rule.title,
      rule_text: rule.text,
      rule_index: ruleIndex,
      required_operator: rule.requiredOperator || '',
      forbidden_operator: rule.forbiddenOperator || '',
      question_count: dailyChallenge.DAILY_QUESTION_COUNT,
      time_limit: rule.timeBonus ? 105 : 150,
      hint_count: rule.id === 'no_undo' ? 2 : 1,
      allow_hint: true,
      puzzles,
    };
  }

  getDailyChallenge(dateKey, dateSeed, options = {}) {
    const day = String(dateKey || '');
    const cacheKey = `daily:${day}`;
    if (this.cache[cacheKey]) return clone(this.cache[cacheKey]);
    let result = dailyChallenge.build(this.generator, day, dateSeed);
    if (!this.validDailyResult(result)) result = this.buildDailyFallback(day, dateSeed);
    if (!this.validDailyResult(result)) {
      this.lastError = `daily challenge unavailable: ${day}`;
      return null;
    }
    const marked = result.puzzles.every((record) => this.remember(cacheKey, record) || this.scopeSet(cacheKey).has(this.key(record.numbers)));
    if (!marked) {
      this.lastError = `daily challenge contains duplicate questions: ${day}`;
      return null;
    }
    const prepared = clone(result);
    prepared.puzzles = prepared.puzzles.map((record) => this.withGenerationMeta(record, 'scheduled_daily_rotation', dateSeed, cacheKey));
    this.cache[cacheKey] = prepared;
    return clone(prepared);
  }

  getEndlessQuestion(questionIndex, runSeed, options = {}) {
    const question = safeInteger(questionIndex, 0);
    const run = normalizeSeed(runSeed);
    const scope = `endless:${run}`;
    const cacheKey = `${scope}:${question}`;
    if (this.cache[cacheKey]) return clone(this.cache[cacheKey]);
    const used = this.scopeSet(scope);
    // endlessMode historically accepts an object map, while the other
    // generators accept Set. Keep the adapter here so both APIs share the
    // same per-run uniqueness state.
    const endlessUsed = Object.create(null);
    used.forEach((key) => { endlessUsed[key] = true; });
    const performance = options.performance || {};
    const seeds = [
      this.seedFor('endless', run, question, 0),
      this.seedFor('endless', run, question, 1),
      this.seedFor('endless', run, question, 2),
    ];
    let result = null;
    for (const seed of seeds) {
      const candidate = endlessMode.generateQuestion(this.generator, question, seed, endlessUsed, performance);
      // endlessMode records the key before returning, so validity is the
      // acceptance check here; checking used again would reject every result.
      if (candidate && this.isVerified(candidate)) {
        result = this.withGenerationMeta(candidate, 'seeded_endless_generation', seed, scope);
        break;
      }
    }
    if (!result && this.generator && typeof this.generator.generatePuzzleSet === 'function') {
      const config = endlessMode.configForQuestion(question, performance);
      const generated = this.generator.generatePuzzleSet(config, 2000 + question, 1, seeds[0], used);
      if (generated[0] && this.isVerified(generated[0])) result = this.withGenerationMeta(generated[0], 'endless_fallback_generation', seeds[0], scope);
    }
    if (!result) {
      this.lastError = `endless question unavailable: ${run}:${question}`;
      return null;
    }
    used.add(this.key(result.numbers));
    this.cache[cacheKey] = result;
    return clone(result);
  }

  getFriendQuestions(roomSeed, options = {}) {
    const seed = normalizeSeed(roomSeed);
    const count = Math.max(1, Number(options.count || friendMatch.QUESTION_COUNT));
    const roundKey = String(options.roundKey || options.round_id || options.match_id || '').trim();
    const generationSeed = roundKey ? hashSeed('friend-round', seed, roundKey) : seed;
    const cacheKey = `friend:${seed}:${count}:${roundKey}`;
    if (this.cache[cacheKey]) return clone(this.cache[cacheKey]);
    const scope = `friend:${seed}`;
    const previousRoundKeys = this.scopeSet(scope);
    let records = [];
    // If the same room starts another round, reroll against the room's recent
    // local history so a rematch is not just the previous list in a new order.
    for (let attempt = 0; attempt < 8 && records.length < count; attempt += 1) {
      const candidateSeed = attempt === 0 ? generationSeed : hashSeed(generationSeed, 'reroll', attempt);
      const candidate = friendMatch.generatePuzzles(this.generator, seed, candidateSeed, count);
      const fresh = candidate.filter((record) => !previousRoundKeys.has(this.key(record.numbers)));
      if (fresh.length > records.length) records = fresh.slice(0, count);
      if (records.length === count) break;
    }
    if (records.length !== count || records.some((record) => !this.isVerified(record))) {
      const config = Object.assign({ min_digit: 1, max_digit: 9, min_solutions: 1, max_solutions: 12 }, options.config || {});
      records = this.generator.generatePuzzleSet(
        Object.assign({}, config, { shuffle_candidates: true }),
        generationSeed % 100000,
        count,
        generationSeed,
        new Set(previousRoundKeys),
      );
    }
    const keys = new Set();
    if (records.length !== count || records.some((record) => {
      const key = this.key(record.numbers);
      if (keys.has(key) || !this.isVerified(record)) return true;
      keys.add(key);
      return false;
    })) {
      this.lastError = `friend questions unavailable: ${seed}`;
      return [];
    }
    const prepared = records.map((record) => this.withGenerationMeta(record, 'seeded_friend_generation', seed, scope));
    prepared.forEach((record) => previousRoundKeys.add(this.key(record.numbers)));
    this.cache[cacheKey] = prepared;
    return clone(prepared);
  }

  getQuestion(mode, options = {}) {
    switch (normalizeMode(mode)) {
      case 'campaign': return this.getCampaignQuestion(options.levelIndex, options.questionIndex, options);
      case 'daily': return this.getDailyChallenge(options.dateKey, options.dateSeed, options);
      case 'endless': return this.getEndlessQuestion(options.questionIndex, options.runSeed, options);
      case 'friend': return this.getFriendQuestions(options.roomSeed, options);
      default:
        this.lastError = `unsupported question mode: ${mode}`;
        return null;
    }
  }
}

function createQuestionService(options) {
  return new QuestionService(options);
}

module.exports = {
  QuestionService,
  createQuestionService,
  RESERVE_NUMBERS,
  hashSeed,
  normalizeSeed,
};
