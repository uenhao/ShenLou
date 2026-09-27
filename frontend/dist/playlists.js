// 蜃楼 TV 前端 · 频道列表页：右侧频道树（收藏/历史/各列表，+号展开）+ 添加目标
async function loadFavorites() {
  try {
    favUrls = new Set((await A().FavoriteURLs()) || []);
  } catch (_) {
    favUrls = new Set();
  }
  refreshFavBadge();
}

// 切换某频道的收藏状态（搜索表/控制条共用）
async function toggleFav(name, url) {
  try {
    const added = await A().ToggleFavorite({ name, url });
    toast(added ? "已加入收藏" : "已移出收藏");
    await loadFavorites();
    await loadPlaylists();
    renderResults();
    updateFavBtn();
  } catch (err) {
    toast(errmsg(err), true);
  }
}

// ---------- 页签切换（左侧导航栏） ----------
function switchTab(name) {
  const t = document.querySelector(`.navitem[data-tab="${name}"]`);
  if (t) t.click();
}

$$(".navitem").forEach((t) => {
  t.onclick = () => {
    $$(".navitem").forEach((x) => x.classList.toggle("active", x === t));
    $$(".tabpane").forEach((p) => p.classList.toggle("active", p.id === "tab-" + t.dataset.tab));
    if (t.dataset.tab === "recordings") loadRecordings(true);
    if (t.dataset.tab === "playlists") renderTree();
    // 频道树只属于频道列表页；其他页左栏改显「正在观看」迷你卡
    document.querySelector("#chtreeCard").classList.toggle("hide", t.dataset.tab !== "playlists");
    document.querySelector("#nowCard").classList.toggle("hide", t.dataset.tab === "playlists");
    updateNowCard();
    if (typeof reportStageRect === "function") setTimeout(reportStageRect, 50);
  };
});

$("#nowGo").onclick = () => switchTab("playlists");

// ---------- 「正在观看」迷你卡（频道列表页以外的左栏占位，避免下半空置） ----------
function updateNowCard() {
  const card = $("#nowCard");
  if (!card || card.classList.contains("hide")) return;
  const sub = $("#nowSub"), name = $("#nowName"), dot = card.querySelector(".dot");
  if (typeof current !== "undefined" && current) {
    name.textContent = current.name;
    name.title = current.name;
    sub.textContent = tvState === "playing" ? "正在播放" : (tvState === "paused" ? "已暂停" : (tvState === "loading" ? "连接中…" : (tvState === "error" ? "播放失败" : "待机")));
    dot.classList.toggle("on", tvState === "playing");
    dot.classList.toggle("err", tvState === "error");
  } else {
    name.textContent = "未在观看";
    sub.textContent = "去「频道列表」选一个频道吧";
    dot.classList.remove("on", "err");
  }
}

// ---------- 频道树 ----------
let treeExpanded = { __fav: false, __hist: false }; // 分组展开态（键：__fav/__hist/列表名）
let stationCache = {};                             // 列表名 → 频道数组（展开时懒加载）
let favCache = [];                                 // FavoriteStations 结果
let histCache = [];                                // PlayHistory 结果

async function loadPlaylists(keepCur = true) {
  try {
    playlists = (await A().ListPlaylists()) || [];
  } catch (e) {
    playlists = [];
    toast("读取频道列表失败：" + errmsg(e), true);
  }
  await loadFavorites();
  await loadTreeData();
  renderTree();
  renderAddTarget();
}

// 拉取收藏/历史的频道明细（树内容）
async function loadTreeData() {
  try { favCache = (await A().FavoriteStations()) || []; } catch (_) { favCache = []; }
  try { histCache = (await A().PlayHistory()) || []; } catch (_) { histCache = []; }
  refreshHistBadge();
  updateResumeBtn();
}

// 展开某分组时确保频道数据就绪
async function ensureGroupChannels(key) {
  if (key === "__fav" || key === "__hist") { await loadTreeData(); return; }
  if (stationCache[key]) return;
  try { stationCache[key] = (await A().GetStations(key)) || []; }
  catch (_) { stationCache[key] = []; }
}

function renderTree() {
  const body = $("#chtreeBody");
  if (!body) return;
  body.innerHTML = "";

  // 收藏 / 历史 固定分组
  body.appendChild(buildGroup("__fav", `${ic("star-fill", 13)} 收藏`, favCache.length, favCache.map((f) => ({
    name: f.name, url: f.url, sub: (f.playlists && f.playlists.length) ? f.playlists.join("、") : "未加入列表",
    ops: ["unfav", "collect"],
  }))));
  body.appendChild(buildGroup("__hist", `${ic("history", 13)} 历史`, histCache.length, histCache.map((h) => ({
    name: h.name, url: h.url, sub: fmtHistTime(h.at), ops: ["collect"],
  })), { clearable: true }));

  // 各播放列表分组
  for (const p of playlists) {
    const chs = stationCache[p.name];
    const guarded = p.name === BUILTIN_PL;
    body.appendChild(buildGroup(p.name, `${ic("layers", 13)} ${esc(p.name)}`, p.count, chs, {
      plName: p.name,
      ops: guarded ? ["addch"] : ["addch", "rename", "del"],
      loading: treeExpanded[p.name] && !chs,
    }));
  }

  // 展开但未加载的分组补数据后重绘
  for (const key of Object.keys(treeExpanded)) {
    if (treeExpanded[key] && !stationCache[key] && key !== "__fav" && key !== "__hist" && playlists.some((p) => p.name === key)) {
      ensureGroupChannels(key).then(renderTree);
    }
  }
}

// 分组节点。rows 为 null 表示尚未加载
function buildGroup(key, titleHtml, count, rows, opt = {}) {
  const g = document.createElement("div");
  g.className = "tgroup";
  const open = !!treeExpanded[key];
  const head = document.createElement("button");
  head.className = "tgroup-head" + (open ? " open" : "");
  head.innerHTML = `
    <span class="tg-tw">${ic(open ? "minus" : "plus", 12)}</span>
    <span class="tg-title">${titleHtml}</span>
    <span class="pl-count">${count || ""}</span>
    ${opt.ops ? `<span class="tg-ops">
      ${opt.ops.includes("addch") ? `<span class="tg-op" data-op="addch" title="添加频道">${ic("plus", 12)}</span>` : ""}
      ${opt.ops.includes("rename") ? `<span class="tg-op" data-op="rename" title="重命名">${ic("pencil", 12)}</span>` : ""}
      ${opt.ops.includes("del") ? `<span class="tg-op" data-op="del" title="删除列表">${ic("trash", 12)}</span>` : ""}
      ${opt.clearable ? `<span class="tg-op" data-op="clear" title="清空历史">${ic("trash", 12)}</span>` : ""}
    </span>` : ""}`;
  head.onclick = (e) => {
    const op = e.target.closest(".tg-op");
    if (op) { e.stopPropagation(); groupOp(op.dataset.op, key); return; }
    treeExpanded[key] = !treeExpanded[key];
    if (treeExpanded[key]) ensureGroupChannels(key).then(renderTree);
    renderTree();
  };
  g.appendChild(head);

  if (open) {
    const gb = document.createElement("div");
    gb.className = "tgroup-body";
    if (opt.loading) {
      gb.innerHTML = `<div class="trow muted">${spinner()} 加载中…</div>`;
    } else if (!rows || rows.length === 0) {
      gb.innerHTML = `<div class="trow muted">（空）</div>`;
    } else {
      for (const r of rows) gb.appendChild(buildRow(r, opt.plName));
    }
    g.appendChild(gb);
  }
  return g;
}

// 频道行：单击观看；悬停操作（收藏星/编辑/删除）
function buildRow(r, plName) {
  const row = document.createElement("div");
  const isCur = isCurUrl(r.url);
  if (isCur) row.className = "trow cur";
  else row.className = "trow";
  const isFav = favUrls.has(r.url);
  row.dataset.url = r.url;
  row.innerHTML = `
    <span class="tr-dot">${isCur ? ic("stop", 11) : ic("play", 11)}</span>
    <span class="tr-main"><span class="tr-name" title="${esc(r.name)}${r.sub ? " · " + esc(r.sub) : ""}">${esc(r.name)}</span>${r.sub ? `<span class="tr-sub">${esc(r.sub)}</span>` : ""}</span>
    <span class="tr-ops">
      <span class="tr-op" data-op="fav" title="${isFav ? "移出收藏" : "加入收藏"}">${isFav ? ic("star-fill", 12) : ic("star", 12)}</span>
      <span class="tr-op" data-op="collect" title="添加到列表">${ic("plus", 12)}</span>
      ${plName ? `<span class="tr-op" data-op="move" title="迁移到其他列表">${ic("move", 12)}</span>
      <span class="tr-op" data-op="edit" title="编辑">${ic("pencil", 12)}</span>
      <span class="tr-op" data-op="del" title="删除">${ic("trash", 12)}</span>` : ""}
    </span>`;
  row.onclick = (e) => {
    const op = e.target.closest(".tr-op");
    if (op) { e.stopPropagation(); rowOp(op.dataset.op, r, plName, row); return; }
    if (isCur) stopWatching(); else watchChannel({ name: r.name, url: r.url });
  };
  return row;
}

// 频道行操作
async function rowOp(op, r, plName, rowEl) {
  try {
    if (op === "move") return void openMoveModal(plName, r);
    if (op === "fav") return void await toggleFav(r.name, r.url);
    if (op === "collect") return void openCollectModal(r.name, r.url);
    if (op === "edit") {
      formModal({
        title: "编辑频道",
        fields: [
          { key: "name", label: "频道名称", value: r.name },
          { key: "url", label: "流地址", value: r.url, required: true },
        ],
        onOk: async (d, close) => {
          try {
            const idx = stationCache[plName].findIndex((s) => s.url === r.url);
            await A().UpdateStation(plName, idx, { name: d.name, url: d.url });
            close(); toast("已保存");
            delete stationCache[plName];
            await loadPlaylists();
          } catch (err) { toast(errmsg(err), true); }
        },
      });
      return;
    }
    if (op === "del") {
      const idx = stationCache[plName].findIndex((s) => s.url === r.url);
      const removed = stationCache[plName][idx];
      await A().RemoveStation(plName, idx);
      delete stationCache[plName];
      await loadPlaylists();
      toast(`已删除「${removed.name}」`, false, {
        label: "撤销",
        fn: async () => {
          try {
            await A().InsertStation(plName, idx, removed);
            delete stationCache[plName];
            await loadPlaylists();
            toast("已恢复");
          } catch (err) { toast(errmsg(err), true); }
        },
      });
      return;
    }
  } catch (err) { toast(errmsg(err), true); }
}

// 分组操作
function groupOp(op, key) {
  if (op === "clear") {
    confirmModal("清空历史", "将清空全部观看记录，不可恢复。", async () => {
      try {
        await A().ClearHistory();
        histCache = [];
        renderTree(); refreshHistBadge(); updateResumeBtn();
        toast("历史已清空");
      } catch (e) { toast(errmsg(e), true); }
    });
    return;
  }
  if (op === "addch") {
    formModal({
      title: `添加频道到「${key}」`,
      fields: [
        { key: "name", label: "频道名称", placeholder: "如：CCTV-1 综合" },
        { key: "url", label: "流地址（http(s):// 或 m3u8 直链）", required: true, placeholder: "http://…/index.m3u8" },
      ],
      onOk: async (d, close) => {
        try {
          await A().AddStation(key, { name: d.name, url: d.url });
          close(); toast("已添加");
          delete stationCache[key];
          treeExpanded[key] = true;
          await loadPlaylists();
        } catch (err) { toast(errmsg(err), true); }
      },
    });
    return;
  }
  if (op === "rename") {
    formModal({
      title: "重命名列表",
      fields: [{ key: "name", label: "新名称", value: key, required: true }],
      onOk: async (d, close) => {
        if (!d.name || d.name === key) { close(); return; }
        try {
          await A().RenamePlaylist(key, d.name);
          delete stationCache[key];
          treeExpanded[d.name] = true;
          delete treeExpanded[key];
          close(); toast("已重命名");
          await loadPlaylists();
        } catch (err) { toast(errmsg(err), true); }
      },
    });
    return;
  }
  if (op === "del") {
    const p = playlists.find((x) => x.name === key);
    confirmModal("删除列表", `将删除「${p ? p.file : key}」及其中的 ${p ? p.count : "?"} 个频道，不可恢复。`, async () => {
      try {
        await A().DeletePlaylist(key);
        delete stationCache[key];
        delete treeExpanded[key];
        toast("已删除");
        await loadPlaylists();
      } catch (err) { toast(errmsg(err), true); }
    });
    return;
  }
}

// 供定位使用：展开某列表并闪烁频道行
async function openPlaylist(name) {
  treeExpanded[name] = true;
  await ensureGroupChannels(name);
  renderTree();
  flashTreeRow(current ? current.url : "");
}

function flashTreeRow(url) {
  const row = [...document.querySelectorAll(".trow")].find((r) => r.dataset.url === url);
  if (row && row.scrollIntoView) {
    row.scrollIntoView({ block: "nearest" });
    row.classList.remove("flash");
    void row.offsetWidth;
    row.classList.add("flash");
  }
}

// ---------- 搜索页「添加到」下拉 ----------
let targetPinned = false;
function renderAddTarget() {
  const sel = $("#addTarget");
  if (!sel) return;
  const prev = sel.value;
  sel.innerHTML = "";
  if (playlists.length === 0) {
    sel.innerHTML = '<option value="">（先去创建频道列表）</option>';
    return;
  }
  for (const p of playlists) {
    const opt = document.createElement("option");
    opt.value = p.name;
    opt.textContent = p.name;
    sel.appendChild(opt);
  }
  const has = (v) => [...sel.options].some((o) => o.value === v);
  let want = curPl;
  if (targetPinned && has(prev)) want = prev;
  if (want && has(want)) sel.value = want;
}
$("#addTarget").addEventListener("change", () => { targetPinned = true; });

$("#btnNewPl").onclick = () => {
  formModal({
    title: "新建频道列表",
    fields: [{ key: "name", label: "列表名称（数据保存在 ~/Videos/.ShenLou/）", required: true, placeholder: "如：纪录片" }],
    onOk: async (d, close) => {
      if (!d.name) return;
      try {
        await A().CreatePlaylist(d.name);
        close();
        toast("已创建");
        treeExpanded[d.name] = true;
        await loadPlaylists();
      } catch (err) { toast(errmsg(err), true); }
    },
  });
};

// 通用「添加到频道列表」弹窗（搜索结果 / 收藏 / 历史 / 控制条共用）
function openCollectModal(name, url) {
  if (!url) return;
  openModal(`
    <h3>添加到频道列表</h3>
    <p class="confirm-msg">${esc(name)}</p>
    <div class="collect-list">
      ${playlists.length === 0
        ? '<span class="muted">还没有频道列表，请先在右侧「＋」新建</span>'
        : playlists.map((p) => `
          <button class="btn collect-row" data-pl="${esc(p.name)}">
            <span>${ic("tv")}${esc(p.name)}</span><span class="muted">${p.count} 个频道</span>
          </button>`).join("")}
    </div>
    <div class="modal-btns"><button class="btn" id="modalCancel">关闭</button></div>`);
  $("#modalCancel").onclick = closeModal;
  $("#modalBox").querySelectorAll(".collect-row").forEach((row) => {
    row.onclick = async () => {
      try {
        await A().AddStation(row.dataset.pl, { name, url });
        toast(`已添加到「${row.dataset.pl}」`);
        closeModal();
        delete stationCache[row.dataset.pl];
        treeExpanded[row.dataset.pl] = true;
        await loadPlaylists();
      } catch (err) {
        toast(errmsg(err), true);
      }
    };
  });
}

// 迁移频道到其他列表（收藏/历史里的"所属列表"引用随之更新）
function openMoveModal(from, r) {
  const targets = playlists.filter((p) => p.name !== from);
  if (targets.length === 0) { toast("没有其他列表可迁移，先新建一个", true); return; }
  openModal(`
    <h3>迁移频道</h3>
    <p class="confirm-msg">${esc(r.name)} · 从「${esc(from)}」迁往：</p>
    <div class="collect-list">
      ${targets.map((p) => `
        <button class="btn collect-row" data-pl="${esc(p.name)}">
          <span>${ic("tv")}${esc(p.name)}</span><span class="muted">${p.count} 个频道</span>
        </button>`).join("")}
    </div>
    <div class="modal-btns"><button class="btn" id="modalCancel">取消</button></div>`);
  $("#modalCancel").onclick = closeModal;
  $("#modalBox").querySelectorAll(".collect-row").forEach((row) => {
    row.onclick = async () => {
      try {
        const idx = stationCache[from].findIndex((s) => s.url === r.url);
        await A().MoveStationCross(from, idx, row.dataset.pl);
        closeModal();
        toast(`已迁移到「${row.dataset.pl}」`);
        delete stationCache[from];
        delete stationCache[row.dataset.pl];
        treeExpanded[row.dataset.pl] = true;
        await loadPlaylists(); // 会顺带刷新收藏/历史的归属引用
      } catch (err) { toast(errmsg(err), true); }
    };
  });
}
