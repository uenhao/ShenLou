// 蜃楼 TV 前端 · 记录（收藏/历史）：数据加载与角标（视图由频道树渲染；定位跳转在 playlists.js）
function fmtHistTime(iso) {
  const d = new Date(iso);
  if (isNaN(d)) return "";
  const now = new Date();
  const hm = `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
  if (d.toDateString() === now.toDateString()) return `今天 ${hm}`;
  const y = d.getFullYear() === now.getFullYear() ? "" : `${d.getFullYear()}年`;
  return `${y}${d.getMonth() + 1}月${d.getDate()}日 ${hm}`;
}

async function refreshFavBadge() {
  try {
    const n = ((await A().FavoriteStations()) || []).length;
    const b = $("#favBadge");
    if (b) { b.textContent = n; b.hidden = n === 0; }
  } catch (_) {}
}

async function refreshHistBadge() {
  try {
    const n = ((await A().PlayHistory()) || []).length;
    const b = document.querySelector("#histBadge");
    if (b) { b.textContent = n; b.hidden = n === 0; }
  } catch (_) {}
}
