# Game HDR Manager

Windows 桌面端游戏 HDR 管理器。它使用 **Go + Fyne** 构建，不依赖 Python、浏览器、WebView 或本地 HTTP 服务。

从本工具启动游戏时，程序会按以下顺序工作：

```text
读取 HDR 状态 → 必要时开启并验证 HDR → 启动游戏 → 监听游戏进程 → 确认退出 → 安全恢复 HDR
```

## 当前状态

这是第一版可运行预览版，已经包含：

- 深色原生桌面 UI、托盘骨架和活动日志
- Steam AppID 和 Epic manifest 扫描
- 扫描结果确认、游戏编辑和多进程配置
- EXE、Steam URI、Epic URI 启动接口
- 启动游戏前开启并验证 HDR
- 全进程快照、多进程游戏追踪和退出确认
- 旧版 `games_config.json` 自动迁移
- HDR 会话保护：用户手动改 HDR 后不再强行恢复

仍在开发：

- WMI 进程事件加速层（当前使用全量快照兜底）
- 更完整的实时状态卡片和通知
- 开机自启和更细致的托盘控制
- 安装包和自动更新

## 界面方向

界面目标是类似 Clash/CCSwitch 的深色桌面控制台：左侧导航、状态卡片、游戏库和活动时间线。UI 使用 Fyne 原生渲染，不是网页套壳。

## 目录结构

```text
cmd/gamehdrmanager/        程序入口
ui/                         Fyne 原生界面、主题和交互
internal/application/       启动服务、监控运行时、HDR 会话协调
internal/config/             配置加载、迁移、原子保存
internal/discovery/          Steam/Epic 扫描与 EXE 候选
internal/domain/             游戏模型和 HDR 会话状态机
internal/hdr/                Windows HDR 控制器
internal/launcher/           EXE/URI 启动器
internal/monitor/            进程快照、追踪和监控循环
legacy/                      旧版 Python/Tkinter 实现，仅用于迁移参考
examples/                    示例配置，不包含真实用户路径
```

## 开发环境

- Windows 10/11
- Go 1.21+
- MinGW-w64 GCC（Fyne 的 Windows OpenGL 驱动需要 C 编译器）
- Git

推荐安装 [MSYS2](https://www.msys2.org/)，然后打开 **MSYS2 UCRT64** 终端：

```bash
pacman -Syu
pacman -S --needed mingw-w64-ucrt-x86_64-gcc
```

如果官方源较慢，可以把 `C:\msys64\etc\pacman.d\mirrorlist.msys` 和 `mirrorlist.mingw` 中的镜像顺序调整为清华或中科大：

```text
https://mirrors.tuna.tsinghua.edu.cn/msys2/
https://mirrors.ustc.edu.cn/msys2/
```

## 构建

在 PowerShell 中：

```powershell
$env:PATH = "C:\msys64\ucrt64\bin;$env:PATH"
$env:CGO_ENABLED = "1"
go mod download
go test ./...
go build -trimpath -ldflags "-s -w -H=windowsgui" -o .\bin\GameHDRManager.exe .\cmd\gamehdrmanager
```

直接运行 `bin\GameHDRManager.exe` 不会打开黑色命令行窗口；也可以使用根目录的 `启动_Go版.vbs` 隐藏启动。

## 配置与隐私

运行配置保存在 Windows 用户配置目录下的 `GameHDRManager\config.json`，不会写入项目目录。不要把个人游戏路径、日志或编译产物提交到 GitHub；仓库中的 `examples/` 仅是脱敏示例。

## 旧版 Python

`legacy/` 中保留了原 Python/Tkinter 版本，作为迁移参考，不是当前推荐入口。新版只需要 Go、Fyne 和 Windows 的 MinGW-w64 编译环境。

也可以直接运行：

```powershell
$env:PATH = "C:\msys64\ucrt64\bin;$env:PATH"
$env:CGO_ENABLED = "1"
go run .\cmd\gamehdrmanager
```

## 使用流程

1. 打开应用，进入“游戏库”。
2. 点击“扫描 Steam / Epic”，勾选需要添加的游戏。
3. 对扫描出的游戏点击“编辑”，确认实际游戏进程名和启动命令。
4. 点击游戏卡片上的“启动”；程序会先准备 HDR，再启动游戏。
5. 需要自动监控时，在概览页点击“启动监控”。

直接双击 `bin\GameHDRManager.exe` 或 `启动_Go版.vbs` 启动新版。项目中的 `启动_GUI版.bat` 和 `启动_HDR管理器.bat` 是旧 Python 版本入口。

外部启动游戏时，程序只能在发现进程后立即补救 HDR，无法保证早于游戏初始化；推荐通过本工具启动。

## 配置与日志

新版配置默认保存在：

```text
%LOCALAPPDATA%\GameHDRManager\config.json
```

首次启动新版时，如果发现项目目录中的旧 `games_config.json`，会迁移并备份为 `games_config.legacy.json`。配置保存采用临时文件替换，避免程序崩溃导致 JSON 损坏。

## 安全行为

- 不直接写入 HDR 注册表值；只通过 Windows `Win + Alt + B` 机制切换，并在切换后验证。
- 只有本次会话由程序开启 HDR，且用户没有手动接管时，退出才会恢复 HDR。
- 游戏启动失败时，会尝试恢复本次临时开启的 HDR。
- 旧版 Python 文件不会被删除，便于回退和对照。

## 已知限制

- Windows HDR 控制入口和不同显卡驱动的响应可能存在差异，必须在真实 HDR 显示器上验证。
- Steam 扫描只能确定游戏和 AppID，实际游戏进程仍建议用户确认。
- 当前监控使用单次全量进程快照，WMI 事件监听会在后续版本加入。
- 项目目前是预览版，不建议无人值守地修改系统 HDR 设置。

## License

暂未指定许可证。公开到 GitHub 前，请根据你的发布意图补充 LICENSE 文件。
