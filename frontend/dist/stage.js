// 52Hz 电台前端……不对，这是蜃楼 TV 的视频舞台控制器（沿用 52HzRadio 前端架构）
// ========= 视频舞台：<video> 内嵌播放、多路流回退、音量/全屏/收藏/录制 =========
// current: 当前频道 { name, url, urls[], idx }
let current = null;
let tvState = "idle"; // idle | loading | playing | paused | error
let lastHistUrl = null; // 去重：同一频道只记一次历史（换台再记）
let engine = "video";  // video | vlc —— <video> 失败自动切 libVLC 内嵌引擎
let vlcAvail = false, vlcTimer = null, vlcWanted = false;

// 供 core.js 的弹窗逻辑查询（弹窗期间需把原生画面挪开避免盖住弹窗）
function vlcEngineActive() { return engine === "vlc"; }

// 把视频舞台的窗口坐标报给后端（libVLC 子窗口对齐用；布局/窗口变化时都要调）
function reportStageRect() {
  // 视频只属于频道列表页：切到其他页隐藏原生窗口（播放不断），坐标随布局实时同步
  const onWatch = document.querySelector("#tab-playlists").classList.contains("active");
  // 弹窗打开期间原生画面同样让位：2 秒心跳会持续调本函数，
  // 若此处照报真实坐标，画面会在弹窗打开后 ≤2s 内被挪回来盖住弹窗
  const modalOpen = !document.querySelector("#modalMask").hidden;
  if (!onWatch || modalOpen) {
    A().SetVideoRect(0, 0, 0, 0).catch(() => {});
    return;
  }
  const r = document.querySelector(".tv-screen").getBoundingClientRect();
  A().SetVideoRect(Math.round(r.x), Math.round(r.y), Math.round(r.width), Math.round(r.height)).catch(() => {});
}

const tv = () => $("#tv");

// ---------- 观看入口（所有列表的统一播放通道） ----------
// st: {name, url}；urls: 可选的多路流（搜索结果带，播放列表条目只有一路）
async function watchChannel(st, urls) {
  const list = (urls && urls.length ? urls.slice() : []).filter(Boolean);
  if (st.url && !list.includes(st.url)) list.unshift(st.url);
  if (!list.length) { toast("频道地址为空", true); return; }
  current = { name: st.name || st.url, url: list[0], urls: list, idx: 0 };
  lastHistUrl = null;
  resetRetry(); // 用户换台 = 全新会话，断流计数清零
  startCurrentPlayback();
}

function startCurrentPlayback() {
  setEngine("video");
  setTVState("loading");
  tv().src = current.url;
  tv().load();
  updateTVBar();
  tv().play().catch(() => {});
}

// ---------- 断流自动重连：间隔 1s→2s→4s→8s→封顶 10s，直到用户手动停止 ----------
let retryN = 0, retryTimer = null;
function cancelRetry() { if (retryTimer) { clearTimeout(retryTimer); retryTimer = null; } }
function resetRetry() { retryN = 0; cancelRetry(); }
function scheduleStreamRetry() {
  cancelRetry();
  clearInterval(vlcTimer);
  const wait = Math.min(1000 * 2 ** retryN, 10000);
  retryN++;
  vlcWanted = false;
  A().VlcStop().catch(() => {});
  setTVState("loading");
  const badge = $("#tvBadge");
  badge.hidden = false;
  badge.textContent = `断流，${Math.round(wait / 1000)}s 后自动重连（第 ${retryN} 次）`;
  badge.className = "tv-badge";
  toast(`「${current.name}」断流，${Math.round(wait / 1000)}s 后自动重连`, true);
  retryTimer = setTimeout(() => {
    retryTimer = null;
    if (!current) return; // 用户已手动停止
    current.idx = 0;
    current.url = current.urls[0];
    startCurrentPlayback(); // 从头走完整回退链：<video> 各路流 → libVLC
  }, wait);
}

function tryNextStream() {
  if (!current) return false;
  if (current.idx + 1 >= current.urls.length) return false;
  current.idx++;
  current.url = current.urls[current.idx];
  tv().src = current.url;
  tv().load();
  tv().play().catch(() => {});
  updateTVBar();
  return true;
}

function stopWatching() {
  resetRetry(); // 用户手动停止：取消一切自动重连
  current = null;
  setEngine("video");
  tv().removeAttribute("src");
  tv().load();
  setTVState("idle");
  updateTVBar();
  refreshTables();
}

// ---------- 状态机 ----------
function setTVState(s) {
  tvState = s;
  $("#tvIdle").style.display = s === "idle" ? "flex" : "none";
  $("#tvLoading").hidden = s !== "loading";
  const badge = $("#tvBadge");
  if (s === "error") {
    badge.hidden = false;
    badge.textContent = "此流无法内嵌播放，试试「VLC 打开」";
    badge.className = "tv-badge err";
  } else if (s === "playing" && current && current.idx > 0) {
    badge.hidden = false;
    badge.textContent = `线路 ${current.idx + 1}/${current.urls.length}`;
    badge.className = "tv-badge";
  } else if (s === "playing" && engine === "vlc") {
    badge.hidden = false;
    badge.textContent = "VLC 引擎";
    badge.className = "tv-badge";
  } else {
    badge.hidden = true;
  }
  $("#tvErrbar").hidden = s !== "error";
  const dot = $("#tvDot");
  dot.className = "dot" + (s === "playing" ? " on" : "");
  updateNowCard(); // 左栏迷你卡（非频道列表页可见）同步状态
}

function updateTVBar() {
  const has = !!current;
  $("#tvName").textContent = has ? `正在观看：${current.name}` : "未在观看";
  $("#tvName").title = has ? current.name : "";
  const pp = $("#btnPlayPause");
  pp.disabled = !has;
  pp.innerHTML = tvState === "playing" ? `${ic("stop")} 停止` : `${ic("play")} 播放`;
  $("#btnFav").disabled = !has;
  $("#btnVlc").disabled = !has;
  updateRecordBtn();
  updateFavBtn();
}

// 各列表刷新当前频道高亮（替换 52HzRadio 的轮询驱动）
function refreshTables() {
  if (document.querySelector('#tab-playlists').classList.contains('active')) renderTree();
  if (document.querySelector('#tab-search').classList.contains('active')) renderResults();
}

// ---------- 视频元素事件 ----------
tv().addEventListener("playing", () => {
  resetRetry(); // 播放成功，断流退避计数清零
  setTVState("playing");
  updateTVBar();
  if (current && lastHistUrl !== current.url) {
    lastHistUrl = current.url;
    A().RecordHistory(current.name, current.url).catch(() => {});
    refreshHistBadge();
  }
  refreshTables();
});
tv().addEventListener("pause", () => { if (tvState === "playing") { setTVState("paused"); updateTVBar(); } });
tv().addEventListener("waiting", () => {
  if (current && document.querySelector("#tab-playlists").classList.contains("active")) setTVState("loading");
});
tv().addEventListener("error", () => {
  if (!current) return;
  // 多路流自动回退
  if (tryNextStream()) {
    toast(`换一路流重试（${current.idx + 1}/${current.urls.length}）`);
    return;
  }
  // <video> 全部失败 → libVLC 内嵌引擎接管（后端会懒初始化，不依赖启动快照）
  setEngine("vlc");
  vlcPlayCurrent();
  return;
});

// ---------- libVLC 内嵌引擎 ----------
function setEngine(e) {
  if (engine === e) return;
  if (engine === "vlc" && e === "video") {
    vlcWanted = false;
    clearInterval(vlcTimer);
    A().VlcStop().catch(() => {});
  }
  engine = e;
}

function vlcPlayCurrent() {
  if (!current) return;
  vlcWanted = true;
  setTVState("loading");
  reportStageRect();
  A().VlcPlay(current.url).then(() => {
    if (!vlcWanted) return; // VlcPlay 在途时用户换台/停止：陈旧回调不得给新会话挂轮询
    reportStageRect(); // 接管成功后补报一次坐标（覆盖懒初始化先于坐标到达的时序）
    clearInterval(vlcTimer);
    vlcTimer = setInterval(async () => {
      if (!vlcWanted) return clearInterval(vlcTimer);
      let s = "idle";
      try { s = await A().VlcState(); } catch (_) { return; }
      if (!vlcWanted || !current) return; // await 期间用户可能已换台/停止：继续跑会误伤新会话（空指针/改错 src）
      if (s === "error") {
        clearInterval(vlcTimer);
        if (tryNextStream()) {
          toast(`VLC 引擎换一路流重试（${current.idx + 1}/${current.urls.length}）`);
          vlcPlayCurrent();
        } else {
          scheduleStreamRetry(); // 全部流都断了：进入退避自动重连，不停止播放会话
        }
      } else if (s === "playing" && tvState !== "playing") {
        resetRetry(); // 播放成功，断流退避计数清零
        setTVState("playing");
        updateTVBar();
        if (lastHistUrl !== current.url) {
          lastHistUrl = current.url;
          A().RecordHistory(current.name, current.url).catch(() => {});
          refreshHistBadge();
          refreshTables();
        }
      } else if (s === "paused") {
        setTVState("paused");
        updateTVBar();
      }
    }, 600);
  }).catch((e) => {
    // 引擎起不来（如未装 libvlc）与"流全部失败"是两回事，只报引擎错误，
    // 不再叠加 finalStreamError 的 toast（单元素 toast 会把前一条顶掉，用户两条都看不全）
    if (!vlcWanted) return; // 在途失败晚到时用户已换台/停止：不得污染新会话的状态机
    cancelRetry();
    vlcWanted = false;
    setTVState("error");
    updateTVBar();
    toast("VLC 引擎：" + errmsg(e), true);
  });
}

// ---------- 控制条 ----------
$("#btnPlayPause").onclick = () => {
  if (!current) return;
  if (engine === "vlc") {
    if (tvState === "paused") { A().VlcSetPaused(false).catch(() => {}); setTVState("playing"); }
    else { A().VlcSetPaused(true).catch(() => {}); }
    updateTVBar();
    return;
  }
  if (tvState === "playing" || tvState === "paused") {
    if (tv().paused) tv().play().catch(() => {});
    else tv().pause();
  } else {
    // loading/error 状态下手动重试当前流：取消挂起的自动重连，避免倒计时到点二次重启
    resetRetry();
    tv().src = current.url;
    tv().load();
    tv().play().catch(() => {});
    setTVState("loading");
    updateTVBar();
  }
};

$("#btnErrVlc").onclick = () => $("#btnVlc").click();
$("#btnVlc").onclick = async () => {
  if (!current) return;
  try { await A().OpenExternalVlc(current.url); } catch (e) { toast(errmsg(e), true); }
};

$("#btnFullscreen").onclick = () => {
  // tv-full 只由 toggleAppFullscreen 挂/摘：一旦引擎回退到 <video>（换台/断流重连），
  // class 仍可能挂着，必须先走整窗退出，否则 Esc/F 在元素全屏分支打转、界面永久卡在全屏形态
  if (document.body.classList.contains("tv-full") || engine === "vlc") return toggleAppFullscreen();
  const el = tv();
  if (document.webkitFullscreenElement) document.webkitExitFullscreen();
  else if (el.webkitRequestFullscreen) el.webkitRequestFullscreen();
  else toast("当前环境不支持全屏");
};

// VLC 引擎全屏：整窗全屏 + 隐藏界面其余部分让舞台铺满（Esc 退出）
async function toggleAppFullscreen() {
  const on = document.body.classList.toggle("tv-full");
  // 舞台只在频道列表页；从其他页按 F 时先切回去，否则全屏出来是空白页
  if (on && !document.querySelector("#tab-playlists").classList.contains("active")) switchTab("playlists");
  try { await (on ? A().FullscreenWindowOn() : A().FullscreenWindowOff()); } catch (_) {}
  reportStageRect();
  // WM 的全屏过渡动画较慢（和最大化同坑），多档补报坐标
  setTimeout(reportStageRect, 300);
  setTimeout(reportStageRect, 900);
}

// 音量（元素音量 0-1；跨曲目记忆）
let volBeforeMute = 100;
function applyVolume(v) {
  v = Math.max(0, Math.min(100, v));
  tv().volume = v / 100;
  tv().muted = v === 0;
  if (engine === "vlc") { A().VlcSetVolume(v).catch(() => {}); A().VlcSetMute(v === 0).catch(() => {}); }
  $("#volSlider").value = v;
  $("#volSlider").style.setProperty("--fill", v + "%");
  $("#volLabel").textContent = v;
  $("#btnMute").innerHTML = v === 0 ? ic("volume-x", 16) : ic("volume", 16);
  localStorage.setItem("sl-vol", String(v));
}
function setVolume(v, fromUser) {
  v = Math.max(0, Math.min(100, v));
  if (fromUser) volBeforeMute = v > 0 ? v : volBeforeMute;
  applyVolume(v); // 本地元素音量即时生效，无需后端节流
}
$("#volSlider").addEventListener("input", (e) => setVolume(parseInt(e.target.value, 10), true));
$("#btnMute").onclick = () => {
  const cur = parseInt($("#volSlider").value, 10) || 0;
  setVolume(cur > 0 ? 0 : (volBeforeMute || 100), false);
};
$("#volSlider").addEventListener("wheel", (e) => {
  e.preventDefault();
  const cur = parseInt($("#volSlider").value, 10) || 0;
  setVolume(cur + (e.deltaY < 0 ? 5 : -5), true);
}, { passive: false });

// ---------- 收藏 / 录制 ----------
$("#btnFav").onclick = async () => {
  if (!current) return;
  await toggleFav(current.name, current.url);
};

// 控制条收藏星标（当前观看频道）
function updateFavBtn() {
  const btn = $("#btnFav");
  if (!btn) return;
  const isFav = !!current && favUrls.has(current.url);
  btn.disabled = !current;
  btn.innerHTML = isFav ? ic("star-fill", 15) : ic("star", 15);
  btn.classList.toggle("fav", isFav);
  btn.title = isFav ? "移出收藏" : "收藏当前频道";
}

function updateRecordBtn() {
  const btn = $("#btnRecord");
  const recording = !!current && recordings.some((r) => r.active && r.url === current.url);
  btn.disabled = !current;
  btn.innerHTML = recording ? `${ic("stop")} 停止录制` : `${ic("circle-dot")} 录制`;
  btn.classList.toggle("recording", recording);
  btn.title = current ? (recording ? "停止当前录制" : "录制当前频道") : "先观看一个频道，才能开始录制";
}

$("#btnRecord").onclick = async () => {
  if (!current) return;
  try {
    const actives = recordings.filter((r) => r.active && r.url === current.url);
    if (actives.length > 0) {
      for (const r of actives) {
        recUserStopped.add(r.id);
        await A().StopRecording(r.id);
      }
      toast("录制已停止");
    } else {
      const info = await A().StartRecording({ name: current.name, url: current.url });
      recActiveSeen.set(info.id, info.name);
      toast("开始录制：" + current.name);
      setTimeout(async () => {
        try {
          const list = await A().ListRecordings();
          if (!list.some((r) => r.active && r.id === info.id)) {
            recUserStopped.add(info.id);
            toast(`录制未能开始：「${info.name}」（流可能已失效）`, true);
          }
        } catch (_) {}
      }, 2500);
    }
    await loadRecordings(true);
  } catch (e) { toast(errmsg(e), true); }
};

// ---------- 待机态：继续观看上次频道 ----------
$("#btnResume").onclick = async () => {
  try {
    const h = (await A().PlayHistory())[0];
    if (h) watchChannel({ name: h.name, url: h.url });
  } catch (_) {}
};


// ---------- libVLC 引擎可用性 + 舞台矩形同步 ----------
(async () => {
  try { vlcAvail = !!(await A().VlcEmbeddedAvailable()); } catch (_) { vlcAvail = false; }
  if (vlcAvail && "ResizeObserver" in window) {
    new ResizeObserver(() => reportStageRect()).observe(document.querySelector(".tv-screen"));
  }
  // 窗口尺寸变化（含最大化/还原，WM 有缩放动画）：多拍补报，动画结束后必定有一帧准的
  window.addEventListener("resize", () => {
    reportStageRect();
    setTimeout(reportStageRect, 300);
    setTimeout(reportStageRect, 900);
  });
})();

// 坐标心跳：跟随录制轮询每 2 秒校准一次原生子窗口（任何漏报场景兜底）
setInterval(() => { if (engine === "vlc") reportStageRect(); }, 2000);
