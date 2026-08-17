/* Canvas visual tokens. UI code may depend on this file; core logic must not. */

const { FONT_SCALE, HOME_TITLE } = require('../config/game_config.js');

const COLORS = {
  bg: '#F8FBFF',
  bg2: '#F0F9FA',
  panel: 'rgba(255, 255, 255, 0.96)',
  panelLight: 'rgba(238, 250, 252, 0.96)',
  text: '#1E2942',
  muted: '#7B879E',
  cyan: '#35C9D1',
  pink: '#F17D9B',
  purple: '#8D78E6',
  gold: '#F4B940',
  green: '#37C995',
  danger: '#F26F71',
};

const GAME_UI = {
  bgTop: '#F7FCFF',
  bgMid: '#F5F9FF',
  bgBottom: '#FFF9F0',
  cyan: '#35C9D1',
  cyanLight: '#1BAEB9',
  cyanDark: '#168C9A',
  magenta: '#F17D9B',
  magentaLight: '#E45D83',
  magentaDark: '#B94E70',
  violet: '#8D78E6',
  violetLight: '#7560D1',
  violetDark: '#6250B2',
  gold: '#F4B940',
  goldLight: '#D9941F',
  goldDark: '#B67718',
  success: '#37C995',
  text: '#1E2942',
  secondary: 'rgba(45,59,86,0.72)',
  muted: 'rgba(83,101,128,0.66)',
  faint: 'rgba(83,101,128,0.34)',
  panelA: 'rgba(255,255,255,0.98)',
  panelB: 'rgba(242,249,255,0.98)',
  modalA: 'rgba(255,255,255,0.99)',
  modalB: 'rgba(244,250,255,0.99)',
  overlay: 'rgba(30,41,66,0.24)',
  radiusLg: 28,
  radiusMd: 22,
  radiusSm: 16,
  edge: 32,
};

const UI_FONT = '"PingFang SC","Microsoft YaHei",sans-serif';

module.exports = { COLORS, GAME_UI, UI_FONT, FONT_SCALE, HOME_TITLE };
