/* Canvas visual tokens. UI code may depend on this file; core logic must not. */

const { FONT_SCALE, HOME_TITLE } = require('../config/game_config.js');

const COLORS = {
  bg: '#0f0d2f',
  bg2: '#25205d',
  panel: 'rgba(29, 25, 83, 0.92)',
  panelLight: 'rgba(59, 49, 137, 0.88)',
  text: '#ffffff',
  muted: '#aeb6ef',
  cyan: '#50e3ff',
  pink: '#ff86b5',
  purple: '#bf9cff',
  gold: '#ffd166',
  green: '#8ee8bd',
  danger: '#ff806d',
};

const GAME_UI = {
  bgTop: '#06072D',
  bgMid: '#10104E',
  bgBottom: '#191664',
  cyan: '#28E9FF',
  cyanLight: '#8CF6FF',
  cyanDark: '#087DB6',
  magenta: '#FF4FCA',
  magentaLight: '#FF99E2',
  magentaDark: '#92216F',
  violet: '#9A64FF',
  violetLight: '#C8A4FF',
  violetDark: '#4B2D99',
  gold: '#FFD34D',
  goldLight: '#FFF097',
  goldDark: '#C58B17',
  success: '#42F3A3',
  text: '#FFFFFF',
  secondary: 'rgba(255,255,255,0.72)',
  muted: 'rgba(210,215,255,0.52)',
  faint: 'rgba(200,205,255,0.35)',
  panelA: 'rgba(37,33,105,0.68)',
  panelB: 'rgba(20,18,76,0.58)',
  modalA: 'rgba(13,12,55,0.97)',
  modalB: 'rgba(20,15,69,0.96)',
  overlay: 'rgba(2,3,24,0.70)',
  radiusLg: 30,
  radiusMd: 24,
  radiusSm: 18,
  edge: 32,
};

const UI_FONT = '"PingFang SC","Microsoft YaHei",sans-serif';

module.exports = { COLORS, GAME_UI, UI_FONT, FONT_SCALE, HOME_TITLE };
