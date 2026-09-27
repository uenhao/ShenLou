#!/bin/bash
# ShenLou deb 打包：./package.sh <版本号>（如 0.0.1）
# 产物：dist/shenlou_<版本>_amd64.deb（Ubuntu 24.04 / Mint 22 及同代发行版）
set -e
cd "$(dirname "$0")"

VER="${1:?用法：./package.sh <版本号>，如 0.0.1}"
PKG="shenlou"
STAGE="dist/stage/${PKG}_${VER}_amd64"

# 确保 fresh 二进制（release 语义：strip + 固定 buildID 用于单实例版本握手）
export PATH="$HOME/.local/go/bin:$HOME/GoPath/bin:$PATH"
export GOPATH="$HOME/GoPath" GOBIN="$HOME/GoPath/bin"
wails build -clean -tags webkit2_41 -ldflags "-s -w -X main.buildID=v${VER}" >/dev/null

rm -rf "$STAGE"
mkdir -p "$STAGE/DEBIAN" \
         "$STAGE/usr/bin" \
         "$STAGE/usr/share/applications" \
         "$STAGE/usr/share/icons/hicolor/scalable/apps" \
         "$STAGE/usr/share/doc/${PKG}"

install -m 0755 build/bin/ShenLou "$STAGE/usr/bin/ShenLou"
# 双大小写文件名：匹配 GTK 按 WM_CLASS（ShenLou / shenlou）查找窗口图标
install -m 0644 icon.svg "$STAGE/usr/share/icons/hicolor/scalable/apps/ShenLou.svg"
install -m 0644 icon.svg "$STAGE/usr/share/icons/hicolor/scalable/apps/shenlou.svg"

cat > "$STAGE/usr/share/applications/ShenLou.desktop" << EOF
[Desktop Entry]
Type=Application
Name=ShenLou
Comment=蜃楼 TV · 搜索、播放列表、VLC 播放与录制
Exec=ShenLou
Icon=ShenLou
Terminal=false
Categories=AudioVideo;Audio;Player;
StartupWMClass=ShenLou
EOF

cat > "$STAGE/DEBIAN/control" << EOF
Package: ${PKG}
Version: ${VER}
Section: sound
Priority: optional
Architecture: amd64
Depends: libwebkit2gtk-4.1-0, libvlc5
Recommends: vlc, gstreamer1.0-plugins-good, gstreamer1.0-plugins-bad, gstreamer1.0-libav
Maintainer: uenhao <smh_dev@163.com>
Description: 网络电视播放器（蜃楼 TV）
 Go + Wails 桌面应用：iptv-org 全球电视台搜索（多路流自动回退）、
 m3u 频道列表管理、收藏与播放历史、内嵌 libVLC 引擎播放与多路并行录制。
 关闭窗口最小化到托盘，观看不中断。
EOF

cat > "$STAGE/DEBIAN/postinst" << 'EOF'
#!/bin/sh
set -e
command -v update-desktop-database >/dev/null && update-desktop-database -q || true
command -v gtk-update-icon-cache >/dev/null && gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor || true
exit 0
EOF
chmod 0755 "$STAGE/DEBIAN/postinst"

cat > "$STAGE/usr/share/doc/${PKG}/copyright" << EOF
Format: https://www.debian.org/doc/packaging-manuals/copyright-format/1.0/
Upstream-Name: ShenLou
Source: https://github.com/uenhao/ShenLou

Files: *
Copyright: 2026 uenhao
License: Apache-2.0
EOF

# 系统目录统一 0755，不跟随构建机 umask
find "$STAGE" -type d -exec chmod 0755 {} +
dpkg-deb --build --root-owner-group "$STAGE" "dist/${PKG}_${VER}_amd64.deb"
rm -rf "$STAGE"
echo
echo "打包完成：$(pwd)/dist/${PKG}_${VER}_amd64.deb ($(du -h "dist/${PKG}_${VER}_amd64.deb" | cut -f1))"
