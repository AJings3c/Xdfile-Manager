# Xdfile Manager - 开发计划

> 双栏终端文件管理器，支持本地文件操作、命令行执行、文件预览、主题切换、宏命令菜单以及 SSH / NetBox 远程目录管理。

---

## 一、项目定位

Xdfile Manager 是一款面向键盘操作的高效终端文件管理器，核心目标是：

- **双栏文件管理** — 左手文件操作，右手命令执行，无需切换窗口
- **命令执行与文件管理一体化** — 内置终端 + 文件面板，命令行与文件操作同屏完成
- **多平台覆盖** — Windows / Linux / macOS
- **远程管理** — SSH / NetBox 远程目录访问
- **高度可定制** — 主题、快捷键、宏命令菜单

---

## 二、技术栈

| 层级 | 技术 |
|---|---|
| **语言** | Go 1.25+ |
| **TUI 框架** | Charmbracelet Bubbletea + Lipgloss + Ultraviolet |
| **终端模拟** | charmbracelet/x/vt (SafeEmulator) |
| **PTY 后端** | golang.org/x/term + 平台原生 (linux/macOS/windows) |
| **语法高亮** | alecthomas/chroma/v2 |
| **图像处理** | golang.org/x/image + disintegration/imaging |
| **剪贴板** | atotto/clipboard |
| **跨平台编译** | GitHub Actions (8 平台) |

---

## 三、当前实现状态

### 3.1 已完成功能

| 功能 | 状态 | 说明 |
|---|---|---|
| 双面板文件管理 | ✅ | 本地文件 + SSH 远程 |
| 文件操作 | ✅ | 复制/移动/删除/重命名/新建目录 |
| 多选 | ✅ | Shift/鼠标/范围选择 |
| 文件预览 | ✅ | 文本/图片/压缩包/PDF/二进制 |
| 主题系统 | ✅ | 5 个内置 Persona 主题 |
| 内置命令 | ✅ | ls/dir/echo/pwd/cd/cat/type |
| 宏命令菜单 | ✅ | F2 用户自定义菜单 |
| SSH / NetBox | ✅ | 远程目录浏览和文件操作 |
| Git 状态 | ✅ | 文件标记 |
| 剪贴板操作 | ✅ | 复制/剪切/粘贴，本地+远程 |
| 搜索 | ✅ | 面板内快速搜索 |
| 终端尺寸自适应 | ✅ | 最低 40x10 |
| CI/CD | ✅ | GitHub Actions 8平台编译 |
| 自动 Release | ✅ | push 到 main 自动发布 |

### 3.2 部分完成功能

| 功能 | 状态 | 问题 |
|---|---|---|
| 内置终端 | ⚠️ | Managed 模式仅支持简单命令，PTY 未默认启用 |
| 交互式程序 | ⚠️ | vim/top/htop 等程序支持不完整 |

### 3.3 缺失功能

| 功能 | 优先级 | 说明 |
|---|---|---|
| 完整 PTY 支持 | P0 | 默认启用 PTY，支持所有交互式程序 |
| 命令自动检测 | P1 | 自动判断使用 Managed 还是 PTY |
| 命令历史 | P1 | 完整命令历史和搜索 |
| 命令自动补全 | P2 | 基于历史的智能补全 |
| 终端分屏 | P2 | 类似 tmux 的多终端 |
| 文件拖拽 | P2 | 鼠标拖拽移动文件 |
| 书签/收藏 | P2 | 常用目录快速跳转 |
| 批量重命名 | P2 | 多文件批量重命名 |
| 右键菜单 | P2 | 上下文菜单 |
| 模糊搜索 | P2 | 命令面板模糊搜索 |
| 插件系统 | P3 | 扩展机制 |
| 内置 AI | P3 | 命令解释/代码生成辅助 |

---

## 四、终端命令执行 - TODO 详细清单

### 4.1 核心目标

> 实现完整的命令执行能力，支持所有交互式程序(vim/top/tmux/htop 等)，无任何功能缺陷。

### 4.2 当前架构分析

```
┌─────────────────────────────────────────────────────┐
│                  Xdfile Manager                      │
├─────────────────────────────────────────────────────┤
│                                                     │
│  ┌─────────────┐     ┌─────────────────────────┐   │
│  │  Managed     │     │  PTY Session            │   │
│  │  Shell Mode  │     │  (Real Terminal)        │   │
│  │             │     │                         │   │
│  │  Built-in:   │     │  Backend (PTY) ──►     │   │
│  │  pwd/echo    │     │  Process               │   │
│  │  cd/ls/dir   │     │       │                │   │
│  │  cat/type    │     │       ▼                │   │
│  │             │     │  VT Emulator            │   │
│  │  ⚠ 简单命令  │     │       │                │   │
│  │  ⚠ 无管道    │     │       ▼                │   │
│  │  ⚠ 无重定向  │     │  Bubble Tea UI         │   │
│  └─────────────┘     └─────────────────────────┘   │
│                                                     │
│  命令执行决策:                                       │
│  - 默认 Managed (简单命令)                          │
│  - 交互式程序 → PTY (未自动检测)                   │
└─────────────────────────────────────────────────────┘
```

### 4.3 TODO 清单

#### P0 - 关键缺陷修复

- [ ] **PTY 默认启用**
  - 文件: `src/cmd/xdfile_terminal_state.go`
  - 修改默认终端模式为 PTY
  - 所有命令通过 PTY 执行

- [ ] **交互式程序自动检测**
  - 文件: `src/cmd/xdfile_shell.go` 或新建 `xdfile_command_classifier.go`
  - 实现 `needsPTY(cmd string) bool` 函数
  - 交互式程序列表: `vim nano emacs less more most top htop btop htop3 gotop tmux screen ranger mc vifm nnn lf vifm irssi weechat mutt neomutt nano joe jupp micro pulsar`
  - 管道命令: `|` 触发 PTY
  - 重定向: `> < >> 2>` 触发 PTY

- [ ] **PTY 独占模式 CWD 同步**
  - 文件: `src/cmd/xdfile_terminal_pty.go`
  - 修改 `sendWorkingDirectory` 支持 exclusive 模式
  - 执行完成后同步工作目录到面板

#### P1 - 重要功能

- [ ] **完整 Shell 语义支持**
  - 文件: `src/cmd/xdfile_shell.go`
  - 支持管道 `|`
  - 支持重定向 `> < >> 2>`
  - 支持命令链 `&& || ;`
  - 支持子 shell `(...)`
  - 支持变量展开 `$VAR ${VAR}`
  - 方案: 所有非内置命令通过真实 shell (bash/zsh/fish/pwsh) 执行

- [ ] **命令历史管理**
  - 文件: `src/cmd/xdfile_terminal_history.go` 或 `xdfile_commands.go`
  - 持久化命令历史到磁盘
  - 历史搜索 (`Ctrl+R`)
  - 历史去重
  - 分 session 保存

- [ ] **命令自动补全**
  - 文件: `src/cmd/xdfile_terminal_suggestions.go`
  - 基于历史的命令补全
  - 基于 PATH 的程序名补全
  - 基于当前目录的文件名补全
  - Shell 原生补全集成

- [ ] **输入键位完整覆盖**
  - 文件: `src/cmd/xdfile_terminal_input.go`
  - Ctrl 组合键 (Ctrl+C/V/Z 等)
  - Alt/Meta 组合键
  - 函数键 F1-F12
  - 方向键完整支持
  - Insert/Delete/PageUp/PageDown

- [ ] **鼠标报告完善**
  - 文件: `src/cmd/xdfile_terminal_pty.go`
  - 确认 vim/emacs mouse 模式支持
  - 确认 tmux screen 模式支持
  - 测试 htop/top/jtop 鼠标交互

#### P2 - 增强功能

- [ ] **终端分屏**
  - 新文件: `src/cmd/xdfile_terminal_splits.go`
  - 类似 tmux 的多窗口管理
  - 水平/垂直分屏
  - 窗口切换
  - 窗口重命名

- [ ] **命令别名管理**
  - 文件: `src/cmd/xdfile_commands.go`
  - 用户自定义别名
  - 别名优先级 (菜单 > 别名 > 内置 > shell)
  - 别名导入/导出

- [ ] **环境变量面板**
  - 新文件: `src/cmd/xdfile_env.go`
  - 查看/编辑当前 shell 环境变量
  - PATH 管理
  - 快速设置临时变量

- [ ] **作业控制**
  - Ctrl+Z 挂起/恢复
  - `jobs` 查看后台任务
  - `fg bg` 任务切换
  - `kill` 终止进程

- [ ] **命令超时管理**
  - 配置命令最大执行时间
  - 超时提示和终止选项
  - 后台命令超时警告

#### P3 - 高级功能

- [ ] **多终端标签页**
  - 新文件: `src/cmd/xdfile_tabs.go`
  - 多个终端标签页
  - 标签页切换
  - 标签页重命名

- [ ] **终端录制回放**
  - 类似 asciinema 的录制功能
  - 支持导出和分享
  - 回放控制 (暂停/快进)

- [ ] **内置 AI 助手**
  - 新文件: `src/cmd/xdfile_ai.go`
  - 命令解释 (类似 iTerm2 AI Chat)
  - 命令建议和补全
  - 输出分析
  - 支持多 LLM 提供商 (OpenAI/Claude/Gemini 等)
  - 需对接 LLM 配置页面 (参考 CodeSentinel 项目)

- [ ] **终端主题增强**
  - 终端独立配色方案
  - 语法高亮主题
  - 字体设置

---

## 五、文件管理 TODO 清单

### P1 - 重要功能

- [ ] **文件拖拽**
  - 鼠标拖拽移动/复制文件
  - 跨面板拖拽
  - 拖拽进度显示

- [ ] **批量重命名**
  - 多文件选择后批量重命名
  - 支持正则表达式
  - 预览更改
  - 撤销支持

- [ ] **书签/收藏夹**
  - 常用目录书签
  - 快速跳转
  - 书签分组

- [ ] **文件关联**
  - 设置默认打开程序
  - 根据扩展名关联

### P2 - 增强功能

- [ ] **右键上下文菜单**
  - 文件/目录右键菜单
  - 与系统右键菜单集成 (Windows)

- [ ] **模糊搜索**
  - 命令面板模糊搜索
  - 文件名模糊匹配

- [ ] **文件过滤器**
  - 按扩展名/大小/日期过滤
  - 保存过滤器预设

- [ ] **缩略图视图**
  - 图片缩略图网格视图
  - 图标视图切换

---

## 六、系统功能 TODO 清单

### P1 - 重要功能

- [ ] **配置热重载**
  - 修改配置后无需重启
  - 实时生效

- [ ] **多语言支持**
  - i18n 框架
  - 中文/英文/日文/韩文

### P2 - 增强功能

- [ ] **插件系统**
  - 插件 API 设计
  - 插件加载器
  - 示例插件

- [ ] **配置同步**
  - 云端配置同步
  - 多设备统一配置

- [ ] **性能优化**
  - 大目录加载优化
  - 虚拟滚动
  - 缓存策略

---

## 七、CI/CD TODO 清单

- [ ] **多架构优化构建**
  - CGO 交叉编译支持
  - UPX 压缩
  - 代码签名 (macOS)

- [ ] **发布流程自动化**
  - CHANGELOG 自动生成
  - 版本号管理
  - GitHub Release 自动发布

- [ ] **测试覆盖**
  - 单元测试 (>80%)
  - 集成测试
  - E2E 测试 (Playwright)

---

## 八、优先级总结

### 当前阶段 (v1.3.x) - 命令执行完善

```
P0 (立即修复):
  1. PTY 默认启用
  2. 交互式程序自动检测
  3. 管道/重定向/命令链支持

P1 (下一版本):
  4. 命令历史和补全
  5. 键位完整覆盖
  6. 鼠标报告完善
```

### 下一阶段 (v1.4.x) - 效率增强

```
  7. 终端分屏
  8. 命令别名管理
  9. 书签系统
 10. 批量重命名
```

### 未来规划 (v2.0) - 智能化

```
 11. 内置 AI 助手
 12. 插件系统
 13. 终端标签页
```

---

## 九、技术债务

- [ ] 清理未使用的常量 (`xdfileCopyright`, `xdfileCreateNoWindow`)
- [ ] 清理未使用的函数 (`xdfileCopyPath`)
- [ ] 统一错误处理
- [ ] 添加单元测试
- [ ] 代码重构 (大文件拆分)
- [ ] 文档完善 (API/插件开发)

---

*最后更新: 2026-06-04*
