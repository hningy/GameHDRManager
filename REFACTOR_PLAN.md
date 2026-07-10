# Game HDR Manager 重构方案

## 1. 目标与边界

将现有 Python/Tkinter 工具重构为一个 Windows 原生桌面应用：单个 EXE、无 Python 运行时、无浏览器、无本地 HTTP 服务、无 WebView。

本次重构的核心目标不是“检测到游戏后切 HDR”，而是：**从本工具启动游戏时，在游戏进程创建前完成 HDR 切换及确认**。这样游戏初始化显示设备时即可感知 HDR。

第一版只管理 Windows HDR 与游戏启动/退出会话；不引入账号、云同步、远程控制或复杂的通用自动化规则。

## 2. 已确认的技术决策

| 范围 | 选择 | 原因 |
| --- | --- | --- |
| 语言 | Go | 编译为单文件 EXE，适合常驻、并发及 Windows 系统调用。 |
| 原生 UI | Fyne v2 | 纯 Go 图形界面，不使用网页、浏览器或 WebView；可自定义深色控制台式界面。 |
| 运行模式 | 单实例、系统托盘常驻 | 主窗口关闭后可隐藏到托盘，监控不会中断。 |
| 配置 | JSON + 原子写入 | 保留可读性，并防止崩溃/断电损坏配置。 |
| 日志 | 本地滚动文件 + 内存事件流 | 前端可展示活动记录，诊断时可导出。 |
| 进程发现 | Windows 进程事件为主，低频快照为兜底 | 不再为每一个游戏反复启动 `tasklist`。 |

> 不采用 Wails、Electron、Vue/React 或任何 Web 页面。Fyne 的界面是原生应用窗口内的图形渲染。

## 3. 现有版本的问题

1. `game_hdr_gui.py` 同时包含 UI、游戏扫描、配置、线程监控、HDR 操作和日志，职责耦合。
2. 每轮轮询会针对每个游戏执行一次 `tasklist`，游戏数量增加时系统进程创建明显增多。
3. HDR 控制存在两套不同实现：快捷键模拟，以及“修改注册表 + 刷新显示 + 快捷键”。需集中封装并做实际机器验证。
4. GUI 启动时会在 HDR 为开时直接写入关闭值；这会覆盖用户原有状态，必须删除。
5. 仅用单个 `process.exe` 既无法可靠启动平台游戏，也无法处理启动器交接、崩溃重启或多进程游戏。
6. 现有“检测到游戏后才开 HDR”的路径无法保证游戏读取到 HDR 能力，不能作为主流程。

## 4. 产品行为

### 4.1 主路径：从本工具启动

```text
用户点击“启动游戏”
  → 读取并记录 HDR 初始状态
  → 如需要，开启 HDR
  → 验证 HDR 已达到目标状态（失败则不启动，提示原因）
  → 启动 EXE / Steam URI / Epic URI
  → 监听该游戏的实际运行进程
  → 所有实际进程确认退出后，等待退出确认延迟
  → 仅在本会话由本工具开启 HDR 且未检测到用户手动接管时，恢复初始状态
```

默认不人为等待 3–10 秒再开启 HDR；HDR 必须先于游戏启动完成。HDR 切换与验证的等待是必要的系统确认时间，不是进程防抖。

### 4.2 外部启动游戏

用户从 Steam、桌面快捷方式等外部渠道启动时：

- 发现目标进程后立即切换 HDR，不加入启动防抖；
- 界面标记为“外部启动，HDR 可能晚于游戏初始化”；
- 仍记录会话并在退出时遵循安全恢复规则；
- 推荐用户将该游戏改为从本工具启动。

### 4.3 退出确认（保留防抖）

退出侧默认确认 15 秒，允许每个游戏设置 5–60 秒：

- 游戏进程退出后，不立即关闭 HDR；
- 在确认窗口内若同一游戏的任一实际进程重新出现，取消恢复；
- 仅确认所有匹配进程持续不存在后才恢复；
- 多个受监控游戏同时运行时，最后一个会话结束前不恢复 HDR。

这可避免启动器交接、反作弊重启和短暂崩溃重启导致 HDR 反复切换。

## 5. 游戏模型与配置迁移

### 5.1 新游戏模型

```json
{
  "id": "uuid",
  "name": "Cyberpunk 2077",
  "enabled": true,
  "source": "steam",
  "launch": {
    "type": "steam_uri",
    "value": "steam://rungameid/1091500"
  },
  "processes": ["Cyberpunk2077.exe"],
  "installPath": "C:/Games/Cyberpunk 2077",
  "hdr": {
    "enableBeforeLaunch": true,
    "restoreOnExit": true
  },
  "exitConfirmSeconds": 15
}
```

`processes` 支持多个实际游戏进程。启动器进程可用于辅助识别，但不能单独作为“游戏已结束”的判断依据。

### 5.2 迁移旧配置

首次启动时读取旧 `games_config.json`：

- 为每条游戏生成稳定 UUID；
- 将旧 `process` 映射为 `processes: [process]`；
- 如旧 `path` 存在，映射为 `launch.type = exe`；
- 保留 `enabled`、轮询间隔和恢复设置的语义；
- 成功后备份原文件为 `games_config.legacy.json`，再写入新配置；
- 迁移失败不覆盖原文件，并在诊断页给出错误。

## 6. 模块设计

```text
cmd/gamehdrmanager/
  main.go                    程序入口、单实例、依赖装配
internal/
  domain/
    game.go                  游戏、启动方式与配置模型
    session.go               HDR 会话状态机与恢复决策
  hdr/
    controller.go            HDR 控制接口
    windows.go               Windows 实现、检测、切换、验证
  monitor/
    watcher.go               进程事件接口
    windows_wmi.go           Windows 进程启动/退出事件
    snapshot.go              低频进程快照兜底
    coordinator.go           多游戏会话协调与退出确认
  launcher/
    launcher.go              EXE、Steam URI、Epic URI 启动
  discovery/
    steam.go                 Steam 库与 AppID/启动项发现
    epic.go                  Epic manifest 发现
    manual.go                手动选择 EXE
  config/
    store.go                 加载、校验、原子保存、迁移
  logging/
    events.go                内存事件总线、文件日志
  tray/
    tray.go                  托盘菜单和窗口生命周期
ui/
  app.go                     Fyne 应用与导航
  dashboard.go               概览
  games.go                   游戏库、编辑、扫描结果确认
  activity.go                活动日志
  settings.go                全局设置与诊断
  theme.go                   深色主题、卡片、状态色、定制控件
tests/
  session_test.go            状态机和恢复规则
  config_test.go             配置迁移与损坏配置
  coordinator_test.go        多游戏、进程重启、退出确认
```

所有 Windows API 调用必须位于 `hdr`、`monitor`、`launcher`、`tray` 等基础设施层；UI 不直接访问注册表、进程或文件系统。

## 7. HDR 会话状态机

每个 HDR 会话保存：初始状态、是否由本工具改动、是否用户接管、关联游戏和状态变化时间。

```text
Idle
  └─ 启动请求 / 外部进程发现 → Preparing
Preparing
  ├─ HDR 确认成功 → Launching 或 Monitoring
  └─ HDR 失败 → Failed（不自动启动游戏）
Launching
  └─ 目标进程出现 → Monitoring
Monitoring
  ├─ 用户手动改 HDR → UserOverride
  └─ 所有游戏退出 → ExitConfirming
ExitConfirming
  ├─ 任一目标进程重现 → Monitoring
  └─ 到期 → Restoring
Restoring
  └─ 完成 / 无需恢复 → Idle
UserOverride
  └─ 所有游戏退出 → Idle（绝不再自动改变 HDR）
```

恢复条件必须同时满足：

1. 用户开启了“退出时恢复”；
2. 会话期间 HDR 的开启动作由本工具完成；
3. 游戏运行期间未发现用户手动接管；
4. 没有其他受监控游戏仍在运行；
5. 退出确认时间已结束。

## 8. 扫描与启动方案

### Steam

- 读取 Steam 安装路径和 `libraryfolders.vdf`；
- 读取 `appmanifest_*.acf` 获取 AppID、名称与安装目录；
- 尽可能使用 `steam://rungameid/<AppID>` 作为启动方式；
- 扫描 EXE 仅用于候选实际进程，扫描结果必须由用户确认。

### Epic

- 读取 Epic `.item` manifest；
- 获取显示名、安装目录和 `LaunchExecutable`；
- 生成 EXE 启动方式并让用户确认实际进程。

### 手动游戏

- 文件选择器选择 EXE；
- 默认从 EXE 文件名生成实际进程；
- 提供“添加额外进程”与“测试启动”按钮。

扫描的目标是生成“可编辑候选项”，不能依靠“目录中最大的 exe”自动做最终决定。

## 9. 原生 UI 方案

深色、紧凑、状态优先，观感参考 Clash/CCSwitch 类的桌面控制台，而不是网页。

- 左侧导航：概览、游戏库、活动、设置；
- 顶部常驻状态：HDR 开/关/未知、监控状态、当前游戏数；
- 概览主操作：`启动监控`、`手动切换 HDR`、`启动最近游戏`；
- 游戏库：平台标识、游戏名、实际进程、启用开关、运行状态、启动按钮；
- 活动页：按时间记录所有决策及失败原因，可复制/导出；
- 设置页：退出确认时间、开机启动、通知、扫描位置和诊断；
- 托盘菜单：显示主窗口、启动/停止监控、手动切 HDR、退出。

所有长操作（HDR 验证、扫描、启动、配置写入）在 goroutine 中执行，通过 UI 线程安全队列刷新页面，禁止阻塞界面。

## 10. 验收标准

### 核心可靠性

- 从本工具启动游戏时，目标游戏进程创建前 HDR 已被确认到目标状态；
- 用户启动前已开启 HDR 时，程序不会关闭或覆盖该状态；
- 用户在游戏中手动改 HDR 后，程序不会在退出时反向覆盖；
- 游戏启动器交接或 15 秒内重启时，不会关闭 HDR；
- 多个游戏同时运行时，最后一个游戏确认退出前不恢复 HDR；
- 配置损坏、HDR 检测失败或启动失败时，给出明确日志和 UI 提示，且不丢失原配置。

### 性能与体验

- 监控稳定运行时不为每个游戏循环创建 `tasklist`；
- 游戏扫描和 HDR 操作不冻结窗口；
- 关闭主窗口后可继续在托盘监控；
- 发行物可在未安装 Python 的 Windows 环境运行。

## 11. 分阶段实施

### 阶段 0：技术验证

1. 建立 Go 模块与 Fyne 空窗口；
2. 单独验证 HDR 检测、切换、切换后验证以及失败表现；
3. 单独验证 Windows 进程事件、Steam URI 与托盘；
4. 在真实 HDR 显示器上记录验证结果，确认后才进入业务开发。

### 阶段 1：核心与配置迁移

1. 实现领域模型、配置加载/迁移/原子保存；
2. 实现 HDR 会话状态机和单元测试；
3. 实现进程监控协调器、退出确认和日志事件流。

### 阶段 2：启动与扫描

1. 实现手动 EXE、Steam URI、Epic EXE 启动；
2. 实现 Steam/Epic 扫描与候选确认；
3. 接入“预开 HDR → 验证 → 启动 → 监控”的完整路径。

### 阶段 3：原生界面与托盘

1. 完成概览、游戏库、活动、设置页面；
2. 完成托盘、通知、开机自启和诊断导出；
3. 进行高 DPI、窗口缩放、异常恢复测试。

### 阶段 4：发布与回归

1. 构建 Windows GUI EXE；
2. 用旧配置、Steam、Epic、手动游戏、多游戏并发进行回归；
3. 编写安装/升级说明与已知限制。

## 12. 已知风险与处理原则

- Windows HDR 没有应被假设为永久稳定的单一控制入口。必须把实现封装在 `hdr.Controller` 后，并保留检测、验证、失败提示和替换实现的能力。
- 外部启动无法在进程创建前保证 HDR 已开启，这是操作系统时序限制；产品应引导用户通过本工具启动。
- 自动扫描无法 100% 判断每个游戏真正的主 EXE；必须允许用户确认和编辑。
- Fyne 的自定义控件需要额外实现。第一版优先保证信息层级、响应性与可靠性，再逐步打磨动画和视觉细节。
