// 与 Godot 的 core/level_catalog.gd 保持同一份设计规则。
// 数组后两项分别是章节底色和章节目标，旧界面仍可继续使用前三项。
const CHAPTERS = [
  ['基础星球', '先熟悉三步操作，稳稳算出 24', '#5ecdf2', '#152d68', '整数与明显解法'],
  ['括号秘境', '学会安排顺序，让括号帮你取胜', '#c69bff', '#34205e', '括号与多步计算'],
  ['连击跑道', '越快越高分，保持你的连胜节奏', '#ff8ea8', '#5a244f', '速度与连击'],
  ['困难星门', '数字范围扩大，解法更加珍贵', '#ffc775', '#5b3c28', '高难度与少解'],
  ['大师终点站', '完成全部章节，成为 24 点大师', '#9beab8', '#1c4e4a', '综合挑战'],
  ['星云实验室', '在变化的数字里寻找稳定路线', '#8de8ff', '#123f62', '多解筛选与节奏'],
  ['彩虹迷宫', '每一步都可能打开新的出口', '#ffb6e1', '#60294e', '顺序判断与组合'],
  ['时空回廊', '在有限时间里完成更少见的解法', '#b8c7ff', '#303a76', '速度与少解'],
  ['极光天台', '挑战更大的数字和更紧的节奏', '#a8f0d0', '#1c5a55', '进阶数字与连击'],
  ['终极方程式', '完成 200 关，成为真正的 24 点大师', '#ffd58a', '#684323', '全规则综合挑战'],
];

function titleForLevel(chapterIndex, chapterLevel, challenge, index) {
  if (challenge) return '挑战关';
  if (chapterIndex === 0 && chapterLevel < 5) return '基础整数';
  if (chapterIndex === 0 && chapterLevel < 10) return '括号进阶';
  if (chapterIndex === 0 && chapterLevel < 15) return '连击冲刺';
  if (chapterIndex === 0) return '困难挑战';
  return [
    '速度训练', '混合运算', '极限数字', '大师挑战', '实验室试点',
    '迷宫路线', '时空竞速', '极光冲刺', '终极方程式',
  ][chapterIndex - 1] || `第 ${index + 1} 关`;
}

function all() {
  const levels = [];
  for (let index = 0; index < 200; index += 1) {
    const chapterIndex = Math.floor(index / 20);
    const chapterLevel = index % 20;
    const [chapterName, chapterSubtitle, chapterColor, chapterTint, chapterGoal] = CHAPTERS[chapterIndex];
    const challenge = index % 5 === 4;
    const difficultyPhase = index < 20 ? 0 : index < 50 ? 1 : index < 100 ? 2 : 3;
    const difficultyProfile = [
      { name: '简单', minDigit: 1, maxDigit: 9, minSolutions: 2, maxSolutions: 999999, minDifficulty: 1, maxDifficulty: 7 },
      { name: '进阶', minDigit: 1, maxDigit: 9, minSolutions: 1, maxSolutions: 24, minDifficulty: 4, maxDifficulty: 12 },
      { name: '困难', minDigit: 2, maxDigit: 9, minSolutions: 1, maxSolutions: 12, minDifficulty: 7, maxDifficulty: 16 },
      // 大师阶段用“少解法 + 高难度分”控制难度，允许少量 1 参与组合，
      // 在 1～9 的整数题库中仍保持足够的独立题目。
      { name: '大师', minDigit: 1, maxDigit: 9, minSolutions: 1, maxSolutions: 4, minDifficulty: 9, maxDifficulty: 20 },
    ][difficultyPhase];
    const config = {
      index,
      title: titleForLevel(chapterIndex, chapterLevel, challenge, index),
      questionCount: challenge ? 5 : 3,
      timeLimit: Math.max(42, (challenge ? 95 : 68) - chapterLevel * 1.25 - chapterIndex * 2.5),
      // 闯关模式统一采用 100 分制，星级门槛由 GameApp 统一解释：60 / 80 / 100。
      targetScore: 100,
      // 连击跨题累计，但不能超过本关题数；否则后半段关卡的三星条件会变成永远无法完成。
      targetCombo: Math.min(challenge ? 5 : 3, 2 + Math.floor(chapterLevel / 5) + Math.min(chapterIndex, 1)),
      isChallenge: challenge,
      allowHint: chapterLevel < 16,
      hintCount: chapterLevel < 5 ? 2 : chapterLevel < 15 ? 1 : 0,
      difficultyPhase,
      difficultyProfile: difficultyProfile.name,
      minDigit: difficultyProfile.minDigit,
      maxDigit: difficultyProfile.maxDigit,
      minSolutions: difficultyProfile.minSolutions,
      maxSolutions: difficultyProfile.maxSolutions,
      minDifficulty: difficultyProfile.minDifficulty,
      maxDifficulty: difficultyProfile.maxDifficulty,
      chapterIndex,
      chapterLevel: chapterLevel + 1,
      chapterName,
      chapterSubtitle,
      chapterGoal,
      chapterColor,
      chapterTint,
      levelStart: chapterIndex * 20 + 1,
      levelEnd: Math.min((chapterIndex + 1) * 20, 200),
    };
    // 同时保留 Godot 使用的下划线字段，便于单机、联机和服务端共享题目配置。
    config.question_count = config.questionCount;
    config.time_limit = config.timeLimit;
    config.target_score = config.targetScore;
    config.target_combo = config.targetCombo;
    config.is_challenge = config.isChallenge;
    config.allow_hint = config.allowHint;
    config.hint_count = config.hintCount;
    config.min_digit = config.minDigit;
    config.max_digit = config.maxDigit;
    config.min_solutions = config.minSolutions;
    config.max_solutions = config.maxSolutions;
    config.min_difficulty = config.minDifficulty;
    config.max_difficulty = config.maxDifficulty;
    config.difficulty_phase = config.difficultyPhase;
    config.difficulty_profile = config.difficultyProfile;
    config.chapter_index = config.chapterIndex;
    config.chapter_level = config.chapterLevel;
    config.chapter_name = config.chapterName;
    config.chapter_subtitle = config.chapterSubtitle;
    config.chapter_goal = config.chapterGoal;
    config.chapter_color = config.chapterColor;
    config.chapter_tint = config.chapterTint;
    levels.push(config);
  }
  return levels;
}

module.exports = { CHAPTERS, all, pageCount: () => 10 };
