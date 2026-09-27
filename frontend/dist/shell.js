// 蜃楼 TV 前端 · 外壳：主题皮肤 / 无边框窗口控制 / 键盘快捷键 / 录制轮询
// ---------- 主题（皮肤）系统 ----------
const THEMES = [
  { id: "aurora",   name: "极光蓝", swatches: ["#7ebeff", "#c4b2ff", "#8caaff"] },
  { id: "sunset",   name: "暮橙",   swatches: ["#ffbe8c", "#ffa096", "#f2acb4"] },
  { id: "sand",     name: "暖沙",   swatches: ["#f0cdaa", "#f8dcbe", "#ebc3cd"] },
  { id: "lavender", name: "暮紫",   swatches: ["#beb6eb", "#d6c0eb", "#c8d2f5"] },
  { id: "graphite", name: "石墨",   swatches: ["#a2b0c4", "#b6c0ce", "#96a5ba"] },
];

function currentTheme() {
  try { return localStorage.getItem("sl-theme") || "aurora"; } catch (_) { return "aurora"; }
}

function applyTheme(id, save = true) {
  document.body.dataset.theme = id;
  if (save) { try { localStorage.setItem("sl-theme", id); } catch (_) {} }
  $$("#modalBox .sw").forEach((el) => el.classList.toggle("active", el.dataset.id === id));
}

function openThemeModal() {
  const cur = currentTheme();
  openModal(`
    <h3>皮肤</h3>
    <div class="swatches">
      ${THEMES.map((t) => `
        <button class="sw ${t.id === cur ? "active" : ""}" data-id="${t.id}">
          <div class="sw-ball">
            ${t.swatches.map((c) => `<span style="background:${c}"></span>`).join("")}
          </div>
          <div class="sw-name">${t.name}</div>
        </button>`).join("")}
    </div>
    <div class="modal-btns"><button class="btn" id="modalCancel">关闭</button></div>`);
  $("#modalCancel").onclick = closeModal;
  $$("#modalBox .sw").forEach((el) => { el.onclick = () => applyTheme(el.dataset.id); });
}

$("#btnTheme").onclick = openThemeModal;

// ---------- 无边框窗口控制 ----------
$("#wcMin").onclick = () => A().MinimiseWindow().catch(() => {});
$("#wcMax").onclick = () => A().ToggleMaximiseWindow().catch(() => {});
$("#wcClose").onclick = () => A().HideWindow().catch(() => {});
$(".topbar").addEventListener("dblclick", (e) => {
  if (e.target.closest("button") || e.target.closest("select") || e.target.closest("input")) return;
  A().ToggleMaximiseWindow().catch(() => {});
});

// ---------- 键盘快捷键 ----------
function showShortcutsModal() {
  openModal(`
    <h3>键盘快捷键</h3>
    <table class="shortcut-table">
      <tr><td><kbd>空格</kbd></td><td>播放 / 暂停当前频道</td></tr>
      <tr><td><kbd>↑</kbd> / <kbd>↓</kbd></td><td>音量 +5 / -5</td></tr>
      <tr><td><kbd>M</kbd></td><td>静音 / 恢复</td></tr>
      <tr><td><kbd>F</kbd></td><td>全屏 / 退出全屏</td></tr>
      <tr><td><kbd>Ctrl</kbd>+<kbd>F</kbd></td><td>跳到搜索页并聚焦关键词</td></tr>
      <tr><td><kbd>Esc</kbd></td><td>关闭弹窗 / 退出全屏</td></tr>
    </table>
    <div class="modal-btns"><button class="btn" id="modalCancel">知道了</button></div>`);
  $("#modalCancel").onclick = closeModal;
}

$("#btnShortcuts").onclick = showShortcutsModal;

document.addEventListener("keydown", (e) => {
  if (e.key === "Escape") {
    if (!$("#modalMask").hidden) { closeModal(); return; }
    if (document.body.classList.contains("tv-full")) { $("#btnFullscreen").click(); return; }
    if (document.webkitFullscreenElement) { document.webkitExitFullscreen(); return; }
  }
  if (e.ctrlKey && e.key.toLowerCase() === "f") {
    if (!$("#modalMask").hidden) return; // 弹窗开着不切页（原实现会在遮罩底下乱切）
    if (document.body.classList.contains("tv-full")) $("#btnFullscreen").click(); // 整窗全屏中先退出，否则切页后是空白
    e.preventDefault();
    switchTab("search");
    $("#qName").focus();
    $("#qName").select();
    return;
  }
  if (e.ctrlKey || e.metaKey || e.altKey) return;
  const tag = (e.target.tagName || "").toUpperCase();
  if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || tag === "BUTTON") return;
  if (!$("#modalMask").hidden) return;
  switch (e.key) {
    case " ":
      e.preventDefault();
      $("#btnPlayPause").click();
      break;
    case "ArrowUp":
      e.preventDefault();
      setVolume((parseInt($("#volSlider").value, 10) || 0) + 5, true);
      break;
    case "ArrowDown":
      e.preventDefault();
      setVolume((parseInt($("#volSlider").value, 10) || 0) - 5, true);
      break;
    case "m":
    case "M":
      $("#btnMute").click();
      break;
    case "f":
    case "F":
      $("#btnFullscreen").click();
      break;
  }
});

// ---------- 录制状态轮询（2 秒；观看状态由 <video> 事件驱动，无需轮询） ----------
setInterval(async () => {
  try {
    await loadRecordings();
  } catch (_) { /* 忽略，下轮再试 */ }
}, 2000);
