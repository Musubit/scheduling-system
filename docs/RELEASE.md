# Release Guide

> 高校智能排课系统 — Windows 便携版发布文档

---

## 发布流程

### 前置条件

| 工具 | 版本要求 |
|------|----------|
| Go | 1.26+ |
| Wails v3 | alpha2.116+ |
| Task runner | go-task |
| Python + uv | 仅 PyInstaller 打包调度器 |
| MinGW-w64 | `windres`（用于 Windows 资源嵌入） |

### 生产构建

```bash
# 一键：构建 scheduler + Wails GUI exe + 打包便携版 ZIP
task package:portable
```

ZIP 文件名版本号来自 `Taskfile.yml` 顶部 `VERSION` var（当前 `0.4.0-alpha`）。
Bump 版本时统一改 Taskfile 一处，`package:portable` 会自动把值透传给
`package-portable.ps1`，产出 `bin/scheduling-system-portable-v<VERSION>.zip`。

如需临时覆盖版本（例如 hotfix 打测试包），也可命令行指定：

```bash
task package:portable VERSION=0.4.0-alpha3
```

### 手动分步

```bash
# 1. 构建 OR-Tools 调度器（可选，SA-only 模式可跳过）
task build:scheduler

# 2. 构建 Windows GUI exe（含 icon + 版本信息 + manifest）
task windows:build

# 3. 打包便携版 ZIP（Version 参数必须显式传，否则用脚本默认值）
powershell -ExecutionPolicy Bypass -File ./build/scripts/package-portable.ps1 -Version 0.4.0-alpha
```

### 手动构建（流程说明）

`task windows:build` 实际执行：

```bash
# 1. 编译前端
npm run build

# 2. 生成 Windows 资源对象（icon + 版本信息 + manifest）
#    注：wails3 generate syso 与 Go 1.26+ 不兼容，改用 windres
windres -i build/windows/version.rc -o wails_windows_amd64.syso -O coff

# 3. 编译 Go + 嵌入 .syso 资源
go build -tags production -trimpath -buildvcs=false \
  -ldflags="-w -s -H windowsgui" \
  -o bin/scheduling-system.exe .

# 4. 清理临时文件
rm wails_windows_amd64.syso
```

### 构建产物

```
bin/
├── scheduling-system.exe               ← 主程序（含 icon + 版本信息）
└── scheduling-system-portable-v*.zip   ← 便携发布包
```

---

## 便携版使用说明

### 首次使用

1. 下载 `scheduling-system-portable-v*.zip`
2. 解压到任意目录（建议英文路径避免潜在编码问题）
3. 双击 `scheduling-system.exe` 启动
4. WebView2 运行时会自动安装首次运行所需资源
5. 系统会自动创建数据目录：`%LOCALAPPDATA%\scheduling-system\`

### 目录结构

```
解压目录/
├── scheduling-system.exe       ← 主程序
└── scheduler/
    └── scheduler.exe            ← OR-Tools 求解器（可选）

首次运行后自动创建：
%LOCALAPPDATA%\scheduling-system\
├── logs\app.log                ← 运行日志
├── config\app.json             ← 配置文件
└── resources\schedule.db       ← SQLite 数据库
```

### 更新版本

- 下载新版 ZIP，解压覆盖旧目录
- **用户数据（数据库、配置）保存在 `%LOCALAPPDATA%\scheduling-system\`**，不会被覆盖
- 如需清理数据，删除 `%LOCALAPPDATA%\scheduling-system\` 即可

---

## SHA256 校验

发布时在 Release Notes 中提供 SHA256 值，用户可在 PowerShell 中验证文件完整性：

```powershell
# 计算下载文件的 SHA256
Get-FileHash .\scheduling-system-portable-v0.3.3.zip

# 与 Release Notes 中公布的值对比
$expected = "A1B2..."
$actual = (Get-FileHash .\scheduling-system-portable-v0.3.3.zip).Hash
$actual -eq $expected  # 返回 True 表示文件完整
```

```bash
# Linux/macOS 同样适用
sha256sum scheduling-system-portable-v0.3.3.zip
```

---

## 安全说明

### Windows Defender / SmartScreen

便携版 **未进行代码签名**（当前为个人开发项目）。

首次运行时可能触发 SmartScreen 警告：

```
Windows protected your PC
Microsoft Defender SmartScreen prevented an unrecognized app from starting.
```

**安全原因：**
- 未签名的 .exe 从 ZIP 解压后运行是常见恶意软件传播路径
- SmartScreen 对无签名、低下载量的可执行文件默认拦截

**解决方法：**
- 点击 **更多信息（More info）** → **仍要运行（Run anyway）**
- 或使用命令行启动：`.\scheduling-system.exe`

发布前生成 SHA256 校验码，用户可验证文件未被篡改。

### 代码签名（商业发布时）

正式商业发布需准备：

```bash
# 1. 购买 EV 或 OV 代码签名证书
# 2. 配置签名变量（build/windows/Taskfile.yml）
SIGN_CERTIFICATE: "path/to/certificate.pfx"
SIGN_THUMBPRINT: "certificate-thumbprint"
TIMESTAMP_SERVER: "http://timestamp.digicert.com"

# 3. 运行签名任务
task windows:build
task windows:sign

# 4. 打包
powershell .../package-portable.ps1
```

---

## 排错指南

| 问题 | 原因 | 解决 |
|------|------|------|
| `scheduler.exe` 未找到 | OR-Tools 调度器未构建 | `task build:scheduler` 或使用 SA-only 模式 |
| WebView2 运行时缺失 | 系统未安装 WebView2 | Windows 10/11 自带，旧系统需手动安装 |
| 黑窗口闪现 | v0.4.0 已修复：主 exe 走 GUI subsystem，solver 子进程用 `CREATE_NO_WINDOW` 隐藏；若仍出现，检查是否误用 DEV 构建 | 必须走 `task package:portable`（生产分支）；DEV=true 会保留 console 便于调试 |
| SmartScreen 拦截 | 未代码签名 | 点击"仍要运行"，或校验 SHA256 |

---

## Release 清单

发布新版本时检查：

- [ ] **版本号同步 5 处**（bump 时必改；info.json 已停用不再计入）：
  1. `Taskfile.yml` 顶部 `VERSION` var —— **ZIP 文件名来源，务必先改**
  2. `wails.json` `version`
  3. `wails.json` `info.productVersion`
  4. `build/windows/version.rc` `FILEVERSION` / `PRODUCTVERSION`（逗号分隔数字段）
  5. `build/windows/version.rc` 字符串 `FileVersion` / `ProductVersion`
- [ ] **assemblyIdentity 版本同步**：`build/windows/wails.exe.manifest` 的 `assemblyIdentity version="X.Y.Z.W"`
  - 必须是 **major.minor.build.revision 四段十进制**（Windows SxS 硬性要求）
  - **不包含** `-alpha` / `-beta` / `-rc` 等 semver 预发布后缀（SxS spec 不识别）
  - 与产品 semver 的 `major.minor.patch` 对齐，`revision` 通常置 `0`（例如产品 `0.4.0-alpha` → manifest `0.4.0.0`）
  - 违反四段规范会触发运行时错误：`The application has failed to start because its side-by-side configuration is incorrect`
- [ ] 产品名 / 描述 / 公司名一致（`wails.json`、`build/windows/version.rc`、`backend/app.go` 的 `Name` / `Description` / `Title`）
- [ ] `go build` 通过
- [ ] `npm run build` 通过
- [ ] `task windows:build` 通过（含 `-H windowsgui`，构建末尾自动校验 exe 为 GUI subsystem）
- [ ] exe 版本信息正确（右键 → 属性 → 详细信息）：ProductName / FileDescription / CompanyName / ProductVersion / FileVersion
- [ ] exe 图标正确显示
- [ ] 双击启动无 CMD 黑窗、开始排课无 solver 闪窗
- [ ] 便携版 ZIP 生成成功
- [ ] SHA256 校验码已记录
- [ ] CHANGELOG.md 已更新
- [ ] Git tag 已创建

### 元数据来源说明

自 v0.4.0-alpha 起 Windows 资源嵌入统一走 **windres**（Epic G3）。原因：`wails3 generate syso` 在 Go 1.26+ 下产出的 VERSIONINFO 无法被 Windows `FileVersionInfo` API 读取（属性页字段全空）。

| exe 属性页字段 | 权威来源 |
|----|----|
| ProductName / FileDescription / CompanyName / LegalCopyright / Comments / ProductVersion / FileVersion | `build/windows/version.rc` → `windres` → `wails_windows_amd64.syso` → Go 链接器嵌入 |
| 应用图标 | `build/windows/icon.ico`（由 `wails3 generate icons` 从 `build/appicon.png` 生成），rc 中 `1 ICON` 引用 |
| DPI awareness + Common Controls v6 | `build/windows/wails.exe.manifest`，rc 中 `1 24` 引用 |
| 窗口标题 / Wails 应用 Name / Description | `backend/app.go` `application.Options` + `WebviewWindowOptions.Title` |
| Wails 工具链元数据（非 exe 嵌入） | `wails.json` |
| Portable ZIP 文件名版本号 | `Taskfile.yml` 顶部 `VERSION` var（Epic G4）|
| `build/windows/info.json` | **已停用**（Epic G3 前 `wails3 generate syso` 消费；保留仅作历史记录，不再影响构建） |

### 发布分支状态

| 分支 | 状态 | 说明 |
|----|----|----|
| Portable ZIP (`task package:portable`) | ✅ 支持 | 内部 alpha 发布的唯一权威路径 |
| NSIS installer (`task windows:package`) | ⚠️ 不在 v0.4.x 支持范围 | `build/windows/nsis/` 保留，但存在多处已知漂移（版本硬编码 0.1.0 / 产品名旧 / 不含 scheduler.exe）；如需商业发行，需专门 Sprint 处理。旧的 `SchedulingSystem-installer.exe` 遗留产物已在 Epic G4 清理 |
