// 蜃楼 TV 前端 · 入口：DOM 就绪后的初始化
window.addEventListener("DOMContentLoaded", async () => {
  if (!A()) {
    document.body.innerHTML = '<div class="fatal">未检测到 Wails 运行时，请在应用窗口中打开本页面</div>';
    return;
  }
  try { applyTheme(localStorage.getItem("sl-theme") || "aurora", false); } catch (_) { applyTheme("aurora", false); }
  window.addEventListener("error", (ev) => {
    try { A().ReportError(String(ev.message || ev.error)); } catch (_) {}
  });
  window.addEventListener("unhandledrejection", (ev) => {
    try { A().ReportError("unhandled rejection: " + errmsg(ev.reason)); } catch (_) {}
  });
  try {
    try { BUILTIN_PL = (await A().BuiltinPlaylistName()) || BUILTIN_PL; } catch (_) {}
    const dir = await A().PlaylistsDir();
    if (dir) {
      $("#dirLabel").innerHTML = ic("folder", 14);
      $("#dirLabel").title = `数据目录：${dir}（频道列表与录制文件）`;
    }
    if (!(await A().VlcAvailable())) {
      toast("未检测到 VLC：录制与「VLC 打开」不可用（sudo apt install vlc）", true);
    }
    // 音量记忆（本地）
    setVolume(parseInt(localStorage.getItem("sl-vol") || "100", 10), false);
    await loadPlaylists(false);
    if (playlists.length) await openPlaylist(playlists[0].name);
    renderResults();
    refreshHistBadge();
    updateResumeBtn();
    loadCategories();
    const rdir = await A().RecordingsDir();
    if (rdir) $("#recDirLabel").textContent = `${rdir}`;
    await loadRecordings(true);
  } catch (e) {
    toast("初始化失败：" + errmsg(e), true);
    try { A().ReportError("init: " + errmsg(e)); } catch (_) {}
  }
});

// 待机页「继续观看」按钮：有历史才显示
async function updateResumeBtn() {
  try {
    const h = ((await A().PlayHistory()) || [])[0];
    const btn = $("#btnResume");
    btn.hidden = !h;
    if (h) {
      $("#tvIdleSub").textContent = `上次观看：${h.name}`;
      btn.innerHTML = `${ic("play", 14)} 继续观看「${esc(h.name.slice(0, 18))}」`;
    }
  } catch (_) {}
}
