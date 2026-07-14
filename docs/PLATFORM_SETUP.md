# 平台开发环境设置

本项目支持 **Windows** 与 **Linux (Ubuntu 26.04+)** 双平台开发与发布。macOS 与 iOS 目录（`build/darwin`、`build/ios`）由 Wails 模板生成但**不作为发布目标**——本仓库不维护 macOS 构建脚本。

## 通用工具链（两平台共需）

- **Go** ≥ 1.21（推荐 1.24+ 以匹配 `go install mvdan.cc/garble@v0.16.0` 的 toolchain 要求）
- **Node.js** ≥ 20 + npm（前端 Vite 5 构建）
- **[uv](https://docs.astral.sh/uv/)**（Python 虚拟环境与依赖管理；`scheduler/` OR-Tools 服务用）
- **Python** 3.11+（uv 自动管理，无需手动装）
- **Wails v3**：`go install github.com/wailsapp/wails/v3/cmd/wails3@latest`
- **[Task](https://taskfile.dev/)**：`go install github.com/go-task/task/v3/cmd/task@latest`

## Windows

已长期支持，无额外说明。构建 / 打包：

```powershell
task build                  # → bin/scheduling-system.exe
task package:portable       # → bin/scheduling-system-portable-v0.5.5.zip
task dev                    # 开发模式 (wails3 dev)
```

发行版（NSIS 安装包 / MSIX）见 `build/windows/Taskfile.yml`。

## Linux (Ubuntu 26.04 主开发目标)

### 系统前置依赖

Wails v3 GTK4 stack + PyInstaller 需要的原生库：

```bash
sudo apt update
sudo apt install -y \
    build-essential pkg-config git curl \
    libgtk-4-dev libwebkitgtk-6.0-dev \
    libx11-dev libxext-dev libxrandr-dev libxi-dev \
    binutils
```

> `libgtk-4-1` + `libwebkitgtk-6.0-4` 已在 `build/linux/nfpm/nfpm.yaml`
> 的 `depends:` 声明为运行时依赖；上面 `-dev` 包只在**构建机**上需要。

### 打包工具（可选，只在需要产出 .deb / .rpm 时）

```bash
go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
```

### 构建 / 打包命令

```bash
task build                       # → bin/scheduling-system  （GOOS 自动检测）
task package                     # → bin/scheduling-system_0.5.5_amd64.deb (默认 FORMAT=deb)
task package FORMAT=rpm          # RPM 包
task package:portable            # → bin/scheduling-system-portable-v0.5.5-linux-x86_64.tar.gz
task dev                         # 开发模式
```

CGO 已默认关闭（`CGO_ENABLED=0`），SQLite 走纯 Go 驱动
(`github.com/glebarez/sqlite`)。如果未来引入需要 CGO 的依赖：

```bash
task build CGO_ENABLED=1
```

需确保 GTK dev 头文件已安装。

## 交叉编译

- **Windows → Linux**：需要 Docker + GTK dev 镜像，暂**不支持**。请直接在 Ubuntu 上原生构建。
- **Linux → Windows**：`build/windows/Taskfile.yml` 内置 Docker + Zig
  cross image (`wails-cross`)，参见其中的 `build:docker` 目标。

## 隔离性自检（v0.5.5 起）

merge 到 main 前必跑：

```bash
task check:scheduling-isolation   # INV-P2 检查
go test ./backend/...             # 全后端测试
npm --prefix frontend run build   # 前端产物
```

`docs/superpowers/specs/2026-07-13-scheduling-dual-mode-design.md` §7 列出
所有 invariant，`scripts/check_scheduling_isolation.sh` 是命令行入口。
