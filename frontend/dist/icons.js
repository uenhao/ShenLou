// 蜃楼 TV 前端 · 图标系统：SVG 图标表 / ic() / 品牌徽标 / 静态注入
const ICONS = {
  play: '<path d="M7 4.8v14.4a.6.6 0 0 0 .9.5l11.6-7.2a.6.6 0 0 0 0-1L7.9 4.3a.6.6 0 0 0-.9.5Z" fill="currentColor" stroke="none"/>',
  stop: '<rect x="6.2" y="6.2" width="11.6" height="11.6" rx="2.2" fill="currentColor" stroke="none"/>',
  star: '<path d="m12 3 2.7 5.5 6 .9-4.3 4.2 1 6-5.4-2.8-5.4 2.8 1-6L3.3 9.4l6-.9Z"/>',
  "star-fill": '<path d="m12 3 2.7 5.5 6 .9-4.3 4.2 1 6-5.4-2.8-5.4 2.8 1-6L3.3 9.4l6-.9Z" fill="currentColor"/>',
  pencil: '<path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/><path d="m15 5 4 4"/>',
  trash: '<path d="M3 6h18"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6"/><path d="M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/><line x1="10" x2="10" y1="11" y2="17"/><line x1="14" x2="14" y1="11" y2="17"/>',
  plus: '<path d="M5 12h14"/><path d="M12 5v14"/>',
  search: '<circle cx="11" cy="11" r="7.5"/><path d="m21 21-4.3-4.3"/>',
  tv: '<rect x="2.5" y="7" width="19" height="13" rx="2.5"/><path d="m8 2 4 5 4-5"/>',
  maximize: '<path d="M8 3H5a2 2 0 0 0-2 2v3"/><path d="M21 8V5a2 2 0 0 0-2-2h-3"/><path d="M3 16v3a2 2 0 0 0 2 2h3"/><path d="M16 21h3a2 2 0 0 0 2-2v-3"/>',
  external: '<path d="M15 3h6v6"/><path d="M10 14 21 3"/><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>',
  music: '<path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/>',
  "circle-dot": '<circle cx="12" cy="12" r="9.5"/><circle cx="12" cy="12" r="2" fill="currentColor" stroke="none"/>',
  minus: '<path d="M5 12h14"/>',
  square: '<rect x="5.5" y="5.5" width="13" height="13" rx="2"/>',
  x: '<path d="M18 6 6 18"/><path d="m6 6 12 12"/>',
  palette: '<circle cx="13.5" cy="6.5" r="1.6" fill="currentColor" stroke="none"/><circle cx="17.5" cy="10.5" r="1.6" fill="currentColor" stroke="none"/><circle cx="8.5" cy="7.5" r="1.6" fill="currentColor" stroke="none"/><circle cx="6.5" cy="12.5" r="1.6" fill="currentColor" stroke="none"/><path d="M12 2a10 10 0 0 0 0 20c1.1 0 2-.9 2-2v-1c0-1.1.9-2 2-2h1a3 3 0 0 0 3-3 10 10 0 0 0-8-12Z"/>',
  keyboard: '<rect width="20" height="16" x="2" y="4" rx="2.5"/><path d="M6 8h.01"/><path d="M10 8h.01"/><path d="M14 8h.01"/><path d="M18 8h.01"/><path d="M8 12h.01"/><path d="M12 12h.01"/><path d="M16 12h.01"/><path d="M7 16h10"/>',
  volume: '<path d="M11 5 6 9H2v6h4l5 4z"/><path d="M15.5 8.5a5 5 0 0 1 0 7"/><path d="M19 5a10 10 0 0 1 0 14"/>',
  "volume-x": '<path d="M11 5 6 9H2v6h4l5 4z"/><line x1="22" x2="16" y1="9" y2="15"/><line x1="16" x2="22" y1="9" y2="15"/>',
  download: '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" x2="12" y1="15" y2="3"/>',
  grip: '<circle cx="9" cy="5" r="1.4" fill="currentColor" stroke="none"/><circle cx="15" cy="5" r="1.4" fill="currentColor" stroke="none"/><circle cx="9" cy="12" r="1.4" fill="currentColor" stroke="none"/><circle cx="15" cy="12" r="1.4" fill="currentColor" stroke="none"/><circle cx="9" cy="19" r="1.4" fill="currentColor" stroke="none"/><circle cx="15" cy="19" r="1.4" fill="currentColor" stroke="none"/>',
  window: '<rect x="2.5" y="4" width="19" height="16" rx="2.5"/><path d="M2.5 8.5h19"/><path d="M6.5 4v4.5"/><path d="M10.5 4v4.5"/>',
  folder: '<path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/>',
  loader: '<path d="M21 12a9 9 0 1 1-6.219-8.56"/>',
  history: '<path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/><path d="M12 7v5l4 2"/>',
  move: '<path d="M5 12h14"/><path d="m13 6 6 6-6 6"/>',
  layers: '<path d="m12 2 10 5-10 5L2 7Z"/><path d="m2 17 10 5 10-5"/><path d="m2 12 10 5 10-5"/>',
};

function ic(name, size = 15) {
  return `<svg class="ic" width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${ICONS[name] || ""}</svg>`;
}

// 旋转加载指示（Lucide loader 弧线）
function spinner(size = 13) {
  return `<svg class="ic spin" width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true">${ICONS.loader}</svg>`;
}

// 品牌徽标：海上蜃楼（渐变圆角方块 + 白色层叠楼阁 + 底部波浪线）
const BRAND_LOGO = `<svg class="logo" width="21" height="21" viewBox="0 0 64 64" aria-hidden="true">
<defs><linearGradient id="slg" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#5aa7ff"/><stop offset="1" stop-color="#2b4fd0"/></linearGradient></defs>
<rect x="2" y="2" width="60" height="60" rx="14" fill="url(#slg)"/>
<path fill="#ffffff" d="M 20 40 L 20 24 L 26 24 L 26 18 L 32 18 L 32 13 L 38 13 L 38 18 L 44 18 L 44 40 Z"/>
<rect x="28.5" y="22" width="3.4" height="4" fill="#2b4fd0"/>
<rect x="34.5" y="27" width="3.4" height="4" fill="#2b4fd0"/>
<rect x="28.5" y="32" width="3.4" height="4" fill="#2b4fd0"/>
<path d="M 10 47 Q 16 44 22 47 T 34 47 T 46 47 T 58 47" stroke="#fff" stroke-width="3" fill="none" stroke-linecap="round"/>
<path d="M 10 53 Q 16 50 22 53 T 34 53 T 46 53 T 58 53" stroke="#ffffff" stroke-opacity="0.55" stroke-width="3" fill="none" stroke-linecap="round"/>
</svg>`;

// 静态图标注入（顶栏、视频控制条固定按钮）
(function applyStaticIcons() {
  const brand = document.querySelector(".brand");
  if (brand) brand.innerHTML = `${BRAND_LOGO} 蜃楼 <span class="sub">TV</span>`;
  const set = (sel, html) => { const el = document.querySelector(sel); if (el) el.innerHTML = html; };
  set("#btnShortcuts", `${ic("keyboard")} 快捷键`);
  set("#btnTheme", `${ic("palette")} 皮肤`);
  set("#wcMin", ic("minus", 13));
  set("#wcMax", ic("square", 12));
  set("#wcClose", ic("x", 13));
  set("#btnMute", ic("volume", 16));
  set("#btnFullscreen", `${ic("maximize", 14)} 全屏`);
  set("#btnVlc", `${ic("external", 14)} VLC 打开`);
  set("#btnFav", ic("star", 15));
  set("#btnRecord", `${ic("circle-dot")} 录制`);
  set("#btnPlayPause", `${ic("play")} 播放`);
  set("#btnResume", `${ic("play", 14)} 继续观看上次的频道`);
  set("#btnRefreshIptv", `${ic("download", 13)} 刷新频道库`);
  const idleLogo = document.querySelector("#tvIdleLogo");
  if (idleLogo) idleLogo.innerHTML = BRAND_LOGO.replace('width="21" height="21"', 'width="56" height="56"');
})();
