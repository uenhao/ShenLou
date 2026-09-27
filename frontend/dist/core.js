// 蜃楼 TV 前端（沿用 52HzRadio 架构，经典脚本共享全局作用域，按 index.html 顺序加载）
// ========= 核心：通用助手 / 全局状态 / toast / 弹窗 =========
// RadioHub（52Hz）前端逻辑：全部通过 Wails 绑定调用 Go 后端
const A = () => window.go && window.go.main && window.go.main.App;

const $ = (sel) => document.querySelector(sel);
const $$ = (sel) => document.querySelectorAll(sel);

function esc(s) {
  return String(s ?? "").replace(/[&<>"']/g, (c) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  })[c]);
}

function errmsg(e) {
  return e && e.message ? e.message : String(e);
}
let toastTimer;
function toast(msg, isErr = false, action = null) {
  const t = $("#toast");
  t.innerHTML = esc(msg) + (action ? ` <button class="toast-action">${esc(action.label)}</button>` : "");
  t.classList.toggle("err", isErr);
  t.hidden = false;
  const btn = t.querySelector(".toast-action");
  if (btn && action) btn.onclick = () => { t.hidden = true; action.fn(); };
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => (t.hidden = true), action ? 5000 : 2800);
}

// ---------- 状态 ----------
let playlists = [];    // [{name, file, count}]
let curPl = null;      // 当前选中的播放列表名
let curStations = [];  // 当前列表的频道
let favUrls = new Set(); // 收藏列表中的流地址集合

// 内置默认列表名：与后端 builtinPlaylistName 保持单一真值（boot 时从后端校正）
var BUILTIN_PL = "默认列表";

// 当前正在观看的频道判断（current 在 stage.js 声明，运行时引用）
const isCurUrl = (u) => typeof current !== "undefined" && !!current && current.url === u;
// ---------- 通用弹窗 ----------
function openModal(html) {
  $("#modalBox").innerHTML = html;
  $("#modalMask").hidden = false;
  // VLC 引擎的 X11 子窗口浮在 webview 之上，会把居中的弹窗整个盖住：
  // 弹窗期间先把原生画面挪到屏幕外（声音不断），关闭时再恢复
  if (typeof vlcEngineActive === "function" && vlcEngineActive()) {
    A().SetVideoRect(0, 0, 0, 0).catch(() => {});
  }
}
function closeModal() {
  $("#modalMask").hidden = true;
  $("#modalBox").innerHTML = "";
  if (typeof vlcEngineActive === "function" && vlcEngineActive() && typeof reportStageRect === "function") {
    setTimeout(reportStageRect, 50);
  }
}

// fields: [{key, label, value, type, placeholder, required}]
function formModal({ title, fields, okText = "确定", onOk }) {
  openModal(`
    <h3>${esc(title)}</h3>
    <form id="modalForm">
      ${fields.map((f) => `
        <label class="field"><span>${esc(f.label)}</span>
          <input type="${f.type || "text"}" name="${esc(f.key)}" value="${esc(f.value || "")}"
                 ${f.placeholder ? `placeholder="${esc(f.placeholder)}"` : ""}
                 ${f.required ? "required" : ""}
                 ${f.maxlength ? `maxlength="${f.maxlength}"` : ""}>
        </label>`).join("")}
    </form>
    <div class="modal-btns">
      <button class="btn" id="modalCancel">取消</button>
      <button class="btn primary" id="modalOk">${esc(okText)}</button>
    </div>`);
  const submit = () => {
    const data = {};
    new FormData($("#modalForm")).forEach((v, k) => (data[k] = String(v).trim()));
    onOk(data, closeModal);
  };
  $("#modalCancel").onclick = closeModal;
  $("#modalForm").onsubmit = (e) => { e.preventDefault(); submit(); };
  $("#modalOk").onclick = submit;
  const first = $("#modalBox").querySelector("input");
  if (first) first.focus();
}

function confirmModal(title, msg, onOk) {
  openModal(`
    <h3>${esc(title)}</h3>
    <p class="confirm-msg">${esc(msg)}</p>
    <div class="modal-btns">
      <button class="btn" id="modalCancel">取消</button>
      <button class="btn danger" id="modalOk">确认删除</button>
    </div>`);
  $("#modalCancel").onclick = closeModal;
  $("#modalOk").onclick = () => { closeModal(); onOk(); };
}

