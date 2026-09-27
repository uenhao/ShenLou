// 52Hz 频道前端（由 app.js 按职责拆分，经典脚本共享全局作用域，按 index.html 顺序加载）
// ========= 录制回听页签：录制列表 / 回听 / 导出 / 删除 / 录制按钮 =========
// ---------- 录制 ----------
let recordings = [];
let lastRecKey = "";
let recUserStopped = new Set();  // 用户主动停止的录制 id，消失时不告警
let recActiveSeen = new Map();   // 上一轮仍活跃的录制 id → 名称
let recLoadBusy = false;         // 并发护栏：轮询与手动刷新交叠时避免旧快照覆盖新状态

async function loadRecordings(force) {
  if (recLoadBusy) return;
  recLoadBusy = true;
  try {
    const prev = recActiveSeen;
    const list = (await A().ListRecordings()) || [];
    const next = new Map();
    for (const r of list) if (r.active) next.set(r.id, r.name);
    // 活跃录制真正消失时：非用户停止 → 后端录制进程异常退出（流失效/磁盘满等），提示用户。
    // 「用户已停止」标记只在条目确实离开列表后才清除——停止是异步 kill，
    // 刷新时它可能仍显示活跃，提前清标记会让下一轮轮询误报。
    for (const [id, name] of prev) {
      if (!next.has(id)) {
        if (!recUserStopped.has(id)) {
          toast(`录制已异常结束：「${name}」（流可能已失效）`, true);
        }
        recUserStopped.delete(id);
      }
    }
    recActiveSeen = next;
    recordings = list;
  } catch (_) {
    recordings = [];
  }
  recLoadBusy = false;
  const key = JSON.stringify(recordings.map((r) => [r.id, r.size, r.active]));
  updateRecordBtn();
  if (!force && key === lastRecKey) return;
  lastRecKey = key;
  renderRecordings();
}

function fmtBytes(n) {
  if (n >= 1073741824) return (n / 1073741824).toFixed(1) + " GB";
  if (n >= 1048576) return (n / 1048576).toFixed(1) + " MB";
  if (n >= 1024) return (n / 1024).toFixed(0) + " KB";
  return n + " B";
}

function renderRecordings() {
  const tbody = $("#recBody");
  tbody.innerHTML = "";
  $("#recCount").textContent = recordings.length > 0 ? `共 ${recordings.length} 个录制` : "";
  if (recordings.length === 0) {
    tbody.innerHTML = '<tr><td colspan="7" class="muted" style="text-align:center;padding:30px">还没有录制。<br>播放频道后点击底部的「录制」按钮即可录制当前频道</td></tr>';
    return;
  }
  for (const r of recordings) {
    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td><button class="iconbtn${r.active ? " playing" : ""}" data-act="${r.active ? "recstop" : "play"}"
        title="${r.active ? "停止录制" : "回听"}">${r.active ? ic("stop") : ic("play")}</button></td>
      <td class="name" title="${esc(r.name)}">${esc(r.name)}</td>
      <td class="muted">${esc(r.startedAt)}</td>
      <td class="muted">${fmtBytes(r.size)}</td>
      <td class="muted">${esc(r.duration)}</td>
      <td>${r.active ? '<span class="rec-live">● 录制中</span>' : '<span class="muted">已完成</span>'}</td>
      <td class="ops">
        <button class="iconbtn" data-act="save" title="保存到下载文件夹">${ic("download")}</button>
        ${r.active ? "" : `<button class="iconbtn warn" data-act="del" title="删除">${ic("trash")}</button>`}
      </td>`;
    tbody.appendChild(tr);
  }
}

$("#recBody").addEventListener("click", async (e) => {
  const btn = e.target.closest("button[data-act]");
  if (!btn) return;
  const idx = [...$("#recBody").children].indexOf(btn.closest("tr"));
  const r = recordings[idx];
  const act = btn.dataset.act;
  try {
    if (act === "play") {
      await A().OpenExternalVlc(r.file);
    } else if (act === "recstop") {
      recUserStopped.add(r.id);
      await A().StopRecording(r.id);
      toast("录制已停止");
    } else if (act === "save") {
      const dest = await A().ExportRecording(r.id);
      toast("已保存到：" + dest);
    } else if (act === "del") {
      confirmModal("删除录制", `将删除「${r.name}（${r.startedAt}）」的录制文件，不可恢复。`, async () => {
        try {
          await A().DeleteRecording(r.id);
          toast("已删除");
          await loadRecordings(true);
        } catch (err) { toast(errmsg(err), true); }
      });
      return;
    }
    await loadRecordings(true);
  } catch (err) {
    toast(errmsg(err), true);
  }
});


