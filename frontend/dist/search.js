// 蜃楼 TV 前端（沿用 52HzRadio 前端架构，经典脚本共享全局作用域）
// ========= 搜索页签：iptv-org 频道库搜索 / 结果渲染 =========
let lastResults = [];

async function doSearch() {
  const name = $("#qName").value.trim();
  const country = $("#qCountry").value;
  const cat = $("#qCat").value;
  const limit = parseInt($("#qLimit").value, 10) || 60;
  if (!name && !country && !cat) {
    toast("请至少填写一个搜索条件", true);
    return;
  }
  $("#searchStatus").innerHTML = `${spinner()} 搜索中…`;
  const btn = $("#btnSearch");
  btn.disabled = true;
  btn.innerHTML = `${spinner()} 搜索中…`;
  try {
    lastResults = (await A().SearchChannels(name, country, cat, limit)) || [];
    $("#searchStatus").textContent =
      lastResults.length > 0 ? `共 ${lastResults.length} 个频道（点击行直接观看）` : "没有匹配的频道，试试换个关键词";
    renderResults();
  } catch (e) {
    $("#searchStatus").textContent = "";
    toast("搜索失败：" + errmsg(e), true);
  } finally {
    btn.disabled = false;
    btn.innerHTML = `${ic("search")} 搜索`;
  }
}

function renderResults() {
  const tbody = $("#resultBody");
  tbody.innerHTML = "";
  if (lastResults.length === 0) {
    tbody.innerHTML = `<tr><td colspan="5" class="empty-hint">
      <div class="eh-icon">${ic("tv", 20)}</div>
      <div class="eh-title">搜索全球网络电视台</div>
      <div class="eh-sub">数据来自 iptv-org 开放频道库（每日自动缓存）· Ctrl+F 快速跳转</div>
    </td></tr>`;
    return;
  }
  lastResults.forEach((r) => {
    const tr = document.createElement("tr");
    const isCur = current && r.urls.includes(current.url);
    if (isCur) tr.className = "playing-row";
    const isFav = favUrls.has(r.urls[0]);
    const logo = r.logo
      ? `<img class="ch-logo" src="${esc(r.logo)}" loading="lazy" onerror="this.style.visibility='hidden'" alt="">`
      : `<span class="ch-logo ch-logo-empty">${ic("tv", 14)}</span>`;
    tr.innerHTML = `
      <td><button class="iconbtn${isCur ? " playing" : ""}" data-act="watch" title="${isCur ? "停止" : "观看"}">${isCur ? ic("stop") : ic("play")}</button></td>
      <td class="name">${logo}<span class="ch-name" title="${esc(r.name)}（${r.urls.length} 路流）">${esc(r.name)}</span></td>
      <td class="muted">${esc(r.country || "—")}</td>
      <td class="muted tags">${esc((r.categories || []).slice(0, 3).join("、") || "—")}</td>
      <td>
        <button class="iconbtn${isFav ? " fav" : ""}" data-act="fav" title="${isFav ? "移出收藏" : "加入收藏"}">${isFav ? ic("star-fill") : ic("star")}</button>
        <button class="iconbtn" data-act="add" title="添加到频道列表">${ic("plus")}</button>
      </td>`;
    tr.addEventListener("click", (e) => {
      if (e.target.closest("button")) return;
      watchChannel({ name: r.name, url: r.urls[0] }, r.urls);
      switchTab("playlists"); // 视频画面在频道列表页：点了就看
      autoSaveToDefault(r);
    });
    tbody.appendChild(tr);
  });
  $("#resultBody").querySelectorAll("button[data-act]").forEach((btn) => {
    const r = lastResults[[...$("#resultBody").children].indexOf(btn.closest("tr"))];
    btn.onclick = async (e) => {
      e.stopPropagation();
      const act = btn.dataset.act;
      if (act === "watch") {
        // 与行高亮同一判定（includes）：正在播的是 fallback 线路时也视为"该频道在播"，点击=停止
        if (current && r.urls.includes(current.url)) stopWatching();
        else { watchChannel({ name: r.name, url: r.urls[0] }, r.urls); switchTab("playlists"); autoSaveToDefault(r); }
      } else if (act === "fav") {
        await toggleFav(r.name, r.urls[0]);
      } else if (act === "add") {
        openCollectModal(r.name, r.urls[0]);
      }
    };
  });
}

// 分类下拉（iptv-org 全量分类）
async function loadCategories() {
  try {
    const cats = (await A().IptvCategories()) || [];
    const sel = $("#qCat");
    for (const c of cats) {
      const opt = document.createElement("option");
      opt.value = c;
      opt.textContent = c;
      sel.appendChild(opt);
    }
  } catch (_) { /* 分类加载失败不致命，仍有国家/关键词可用 */ }
}

$("#btnSearch").onclick = doSearch;
$("#qName").addEventListener("keydown", (e) => { if (e.key === "Enter") doSearch(); });
$("#btnRefreshIptv").onclick = async () => {
  try {
    toast("正在刷新频道库…");
    await A().RefreshIptv();
    toast("频道库已更新");
  } catch (e) { toast(errmsg(e), true); }
};

// 搜索里点播的频道自动放入内置默认列表（已存在则忽略）
async function autoSaveToDefault(r) {
  try {
    await A().AddStation(BUILTIN_PL, { name: r.name, url: r.urls[0] });
    delete stationCache[BUILTIN_PL];
    treeExpanded[BUILTIN_PL] = true;
    if (document.querySelector("#tab-playlists").classList.contains("active")) await loadPlaylists();
  } catch (_) { /* 已在列表中 etc. */ }
}
