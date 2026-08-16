const SKINS = [
  { id: 'classic', name: '星空玻璃', description: '深蓝紫背景与青绿色操作高光', price: 0, theme: { bg: '#1e1b4b', surface: '#25215b', surface_2: '#312e81', card: '#ffffff', card_text: '#17163e', accent: '#fbbf24', blue: '#22d3ee', gold: '#fbbf24', text: '#ffffff', muted: '#a5b4fc' } },
  { id: 'ocean', name: '深海蓝', description: '安静、清晰，适合夜间挑战', price: 360, requirement_text: '拥有足够金币即可兑换', theme: { bg: '#102b4f', surface: '#19436c', surface_2: '#245b87', card: '#e9f6ff', card_text: '#123f65', accent: '#75e3ff', blue: '#3188b8', gold: '#e2bd62', text: '#f3fbff', muted: '#b5d4e6' } },
  { id: 'sunset', name: '落日橙', description: '热烈的冲刺挑战氛围', price: 720, min_level: 25, min_stars: 12, requirement_text: '解锁第 25 关并累计 12 颗星', theme: { bg: '#542b27', surface: '#763d2f', surface_2: '#985039', card: '#fff0d9', card_text: '#683727', accent: '#ffd06b', blue: '#c56549', gold: '#f1bd5b', text: '#fff8ee', muted: '#e8c4aa' } },
  { id: 'candy', name: '糖果星云', description: '粉蓝糖果色，轻松又明亮', price: 980, min_level: 18, min_stars: 8, requirement_text: '解锁第 18 关并累计 8 颗星', theme: { bg: '#42215f', surface: '#7c3f9c', surface_2: '#bb5db8', card: '#fff2ff', card_text: '#5d2d72', accent: '#ff9edb', blue: '#72dcff', gold: '#ffe27a', text: '#fff7ff', muted: '#efc8ff' } },
  { id: 'forest', name: '森林精灵', description: '绿色星光，适合慢慢思考', price: 1280, min_level: 35, min_stars: 18, requirement_text: '解锁第 35 关并累计 18 颗星', theme: { bg: '#123d42', surface: '#17605b', surface_2: '#278b6e', card: '#effff4', card_text: '#145448', accent: '#8ff0bd', blue: '#65e4da', gold: '#ffe284', text: '#f4fff9', muted: '#b9ebd0' } },
  { id: 'aurora', name: '极光幻境', description: '流动的蓝绿极光，稀有主题', price: 1680, min_level: 50, min_stars: 28, requirement_text: '解锁第 50 关并累计 28 颗星', theme: { bg: '#102957', surface: '#175b84', surface_2: '#267c9a', card: '#ebffff', card_text: '#164f68', accent: '#8cf7e9', blue: '#6ac5ff', gold: '#ffe98a', text: '#f2ffff', muted: '#b9e4ff' } },
  { id: 'volcano', name: '火焰方程', description: '炽热红橙，给连击加点气势', price: 2200, min_level: 75, min_stars: 45, requirement_text: '解锁第 75 关并累计 45 颗星', theme: { bg: '#4b1d2b', surface: '#7e2d31', surface_2: '#ba4a32', card: '#fff0dc', card_text: '#6b262a', accent: '#ff9c54', blue: '#ff6e74', gold: '#ffe16d', text: '#fff8ef', muted: '#ffc2a5' } },
  { id: 'royal', name: '皇家紫晶', description: '完成高级挑战后解锁的收藏主题', price: 3000, min_level: 100, min_stars: 72, requirement_text: '解锁第 100 关并累计 72 颗星', theme: { bg: '#21144c', surface: '#4b2688', surface_2: '#754bb8', card: '#f4efff', card_text: '#41226d', accent: '#d5a6ff', blue: '#8bb8ff', gold: '#ffe38a', text: '#fffaff', muted: '#d4c2ff' } },
];

// 外观商品和主题分开管理。它们只影响绘制，不参与题目判断，
// 这样以后接入微信云存档或真实商城时，不需要改动核心玩法。
const COSMETICS = [
  { id: 'card_classic', category: 'card', name: '星空卡片', description: '清晰耐看的默认数字卡面', price: 0, preview: 'classic' },
  { id: 'card_neon', category: 'card', name: '霓虹卡片', description: '数字边框带有青色霓虹高光', price: 240, preview: 'neon' },
  { id: 'card_candy', category: 'card', name: '糖果卡片', description: '粉蓝渐变，适合轻松练习', price: 420, min_level: 8, preview: 'candy' },
  { id: 'operator_classic', category: 'operator', name: '标准运算符', description: '稳定清楚的默认按钮风格', price: 0, preview: 'classic' },
  { id: 'operator_bubble', category: 'operator', name: '泡泡运算符', description: '圆润的彩色运算按钮', price: 260, min_level: 5, preview: 'bubble' },
  { id: 'operator_prism', category: 'operator', name: '棱镜运算符', description: '紫青流光，选中时更醒目', price: 520, min_level: 15, preview: 'prism' },
  { id: 'result_classic', category: 'result', name: '星光结算', description: '简洁的星级结算效果', price: 0, preview: 'classic' },
  { id: 'result_burst', category: 'result', name: '彩虹爆闪', description: '答对后绽放彩色光点', price: 360, min_level: 10, preview: 'burst' },
  { id: 'result_fireworks', category: 'result', name: '小小烟花', description: '三星彩星带来庆祝感', price: 680, min_level: 20, min_stars: 6, preview: 'fireworks' },
];

function all() { return JSON.parse(JSON.stringify(SKINS)); }
function getSkin(skinId) { return all().find((skin) => skin.id === skinId) || all()[0]; }
function themeColors(skinId) { return { ...(getSkin(skinId).theme || {}) }; }
function allCosmetics(category = '') {
  const items = category ? COSMETICS.filter((item) => item.category === category) : COSMETICS;
  return JSON.parse(JSON.stringify(items));
}
function getCosmetic(cosmeticId) {
  return allCosmetics().find((item) => item.id === cosmeticId) || allCosmetics()[0];
}
function totalStars(progress) { return Object.values(progress.levels || {}).reduce((sum, record) => sum + Number(record && record.stars || 0), 0); }
function unlockStatus(skinId, progress) {
  const skin = getSkin(skinId);
  if (Number(progress.unlocked_level || 0) < Number(skin.min_level || 0)) return { unlocked: false, reason: `解锁第 ${skin.min_level} 关后可兑换` };
  if (totalStars(progress) < Number(skin.min_stars || 0)) return { unlocked: false, reason: `还需累计 ${skin.min_stars} 颗星（当前 ${totalStars(progress)}）` };
  return { unlocked: true, reason: '' };
}

function cosmeticUnlockStatus(cosmeticId, progress) {
  const item = getCosmetic(cosmeticId);
  const unlockedLevel = Number(progress && progress.unlocked_level || 0);
  const stars = totalStars(progress || {});
  if (unlockedLevel < Number(item.min_level || 0)) return { unlocked: false, reason: `解锁第 ${item.min_level} 关后可兑换` };
  if (stars < Number(item.min_stars || 0)) return { unlocked: false, reason: `还需累计 ${item.min_stars} 颗星（当前 ${stars}）` };
  return { unlocked: true, reason: '' };
}

function categoryLabel(category) {
  return { card: '数字卡片', operator: '运算符', result: '结算特效' }[category] || '外观';
}

module.exports = {
  all, getSkin, themeColors, unlockStatus, totalStars,
  allCosmetics, getCosmetic, cosmeticUnlockStatus, categoryLabel,
};
