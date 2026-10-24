# SleepDock 项目交接

更新日期：2026-10-24。当前版本：0.1.0。

本文件面向接手开发、测试和发布的工程师。首次阅读建议依次查看本文件、[README](README.md)、[实机验收清单](docs/ACCEPTANCE.md)，再进入对应业务包。

## 1. 产品定位与已确定的约束

SleepDock 是独立的原生定时睡眠应用，使用 Go + MyGO，目标平台为 Windows x64、macOS arm64 和 macOS amd64。它有自己的窗口、托盘、配置目录和发布包。

主界面以 UI 表达功能：模式切换、小时／分钟输入、快捷时长、大号环形倒计时和操作按钮。仅保留必要标签，功能原理、权限和开发信息放在文档中；错误细节由用户点击展开。

维护时需保持以下行为：

- 人工设定时间，提供倒计时和指定时刻；同一实例同时只有一个活动计划。
- 不保持唤醒，不修改系统电源计划；系统提前睡眠时结束本次计划，唤醒后不补执行。
- 活动计时关闭窗口时隐藏到托盘；无活动计时关闭窗口退出。托盘不可用时关闭窗口退出，并保留可查看的错误提示。
- 到时额外保留 60 秒提醒，允许取消和延后；无人操作才请求普通睡眠。
- 退出取消计划；重启只恢复上次输入与主题，不恢复旧计划、不默认开机自启。
- 首版没有 AI 完成检测、AI 调用接口、多工具首页或流程编排。

## 2. 已实现功能与当前交付状态

已实现倒计时、跨午夜的指定时间、15 分钟延后、取消、60 秒提醒、托盘菜单、睡眠失败重试、深浅主题和原子设置存储。Windows 与 macOS 的睡眠适配及三平台打包配置已经落地。

| 验证项目 | 当前结果 |
| --- | --- |
| Go 单元测试、MyGO 内存交互测试 | Linux 云端通过 |
| `go test -race ./internal/...` | Linux 云端通过 |
| `go vet ./...` | 通过 |
| Windows x64 / macOS arm64 / macOS amd64 构建 | 通过，产物架构与 ZIP 完整性已核对 |
| Linux 图形启动 | 在 Xvfb 虚拟显示器中启动，使用无 GPU 回退绘制 |
| Windows / macOS 原生托盘、通知、睡眠与唤醒 | 尚未实机验收 |
| Windows 代码签名、Apple Developer ID 签名及公证 | 尚未配置 |

上述云端启动不代表用户电脑已经启动程序。当前开发路径是 `/workspace/sleepdock`；用户本机应解压对应 ZIP，或在本机安装 Go 后从源码运行。

## 3. 快速接手

工具链固定在 Go 1.27.1、MyGO v0.5.5，依赖见 `go.mod` / `go.sum`。打包脚本只需要 Python 标准库，Windows 可用 `python` 代替 `python3`。

```sh
go test ./...
go test -race ./internal/...
go vet ./...
go run ./cmd/sleepdock --config-dir ./artifacts/dev-config
```

开发时使用独立配置目录。自动测试不会调用真实睡眠接口，离线预览也不具备系统睡眠能力：

```sh
go run ./cmd/preview -out artifacts
go run ./cmd/icon
```

当前云端另装了 `/workspace/.repodock-tools/go/bin/go`，这是环境内工具链位置，不应硬编码进项目或当作用户本机 Go 路径。

## 4. 架构与阅读入口

| 入口 | 职责 |
| --- | --- |
| `cmd/sleepdock/main.go` | 参数、版本、配置路径、桌面启动 |
| `internal/domain/timer.go` | 时间计算、计时状态机、一次性触发与计划身份 |
| `internal/application/controller.go` | 可注入时钟／睡眠／通知／存储，状态协调、异步效果与退出 |
| `internal/desktop/view.go` | 时间输入、运行中、提醒、结果与设置界面 |
| `internal/desktop/theme.go` | 主题、图标、按钮与环形绘制 |
| `internal/desktop/run.go` | MyGO 窗口、托盘、通知、系统睡眠事件与一秒 tick |
| `internal/platform/sleep_*.go` | 各系统真实睡眠接口 |
| `internal/storage/preferences.go` | JSON 原子保存、损坏备份、未来版本保护 |

状态主路径：`idle → running → grace → requesting → completed`。取消进入 `cancelled`，系统请求失败进入 `failed`；失败重试重新进入完整 60 秒 `grace`。

界面通过控制器调用用例，不直接调用电源接口。控制器持有锁保护状态，后台效果通过快照和窗口更新通知回到 MyGO 主线程。系统事件与视图回调不能在 UI 线程等待计时 goroutine 或原生睡眠调用结束。

## 5. 时间与生命周期语义

- 倒计时范围为 1–1440 分钟，使用 `now.Add(duration)` 保留 Go 单调时间；指定时间按本机时区创建目标时刻，当天该时刻已过去或恰好等于当前时刻时安排到次日。
- 输入表示提醒触发时间，预计睡眠时间为触发时间加 60 秒。tick 延迟到达时仍给完整 60 秒，不能直接跳过提醒。
- 计时中延后以原触发时刻为起点；提醒期间延后以当前时刻为起点，均增加 15 分钟，再保留提醒的一分钟。
- `Timer.ID` 随重新安排、延后、取消和重试递增。异步结果必须匹配 ID 与 `requesting` 状态，过期结果不能覆盖新计划。
- `OnSuspend` 和 `OnResume` 均会消费仍活动的计划，避免恢复时再次进入睡眠。
- 托盘隐藏窗口不会结束计时；完整退出停止 ticker、取消控制器上下文，并收尾后台任务。已提交给系统的睡眠请求不能当作仍可取消的普通倒计时。

Windows 使用 `powrprof.dll!SetSuspendState(false, false, false)`，在本进程中申请并恢复所需的电源权限；macOS 使用固定系统命令 `/usr/bin/pmset sleepnow`，不拼接任意 Shell。Linux 实现返回“不支持”，仅用于开发与界面验证。

## 6. 配置与资源

Windows 配置为 `%APPDATA%\SleepDock\settings.json`；macOS 为 `~/Library/Application Support/SleepDock/settings.json`。schema 版本为 1，仅保存 `version`、`theme` 和 `last`。不要将活动计划或已过期的目标时刻加入自动恢复流程。

图片与图标位于 `assets/`，README 图片位于 `docs/assets/`。`cmd/preview` 使用内存后端，不调用真实电源接口。用户原始 Windows 脚本存放于 `references/windows-timer/`，作为行为参考，GUI 运行不依赖这些脚本。

## 7. 构建与发布

```sh
python3 scripts/package.py
python3 scripts/package.py --targets windows/amd64
python3 scripts/package.py --go /absolute/path/to/go
```

脚本经固定版本的 MyGO CLI 生成 Windows 清单／图标、macOS `.app`，收集许可、README 和文档，创建 ZIP 并更新 `dist/SHA256SUMS.txt`。在 Mac 本机构建时默认进行 ad-hoc 签名；正式签名和公证需要另行配置。

GitHub Actions 工作流在 Windows、macOS runner 上执行测试、静态检查、预览与打包，产物上传至 Artifacts。工作流文件存在不等于已经远程运行通过；发布时应记录实际运行链接和结果。

源代码、测试、文档、静态资源、许可和工作流应纳入 Git。`dist/`、`artifacts/`、用户设置与构建缓存被 `.gitignore` 排除；安装包可作为独立交付物发布。

## 8. 下一位维护者优先处理

1. 按实机清单在 Windows 与两种 Mac 架构上验证托盘、通知授权、真实睡眠与恢复，记录系统版本和结果。
2. 在 macOS 首次通知授权、勿扰和锁屏场景下确认提醒窗口仍可用；在 Windows 验证普通用户电源权限与不支持睡眠的设备提示。
3. 验证重试、退出与系统睡眠事件同时发生时没有二次请求或过期窗口回写。
4. 完成签名／公证与发布流程后再更新 README 的分发状态；保持主界面克制，不把验收或技术说明塞入用户流程。
