# 在线 Skill 市场集成

VibeCoding 兼容市面上的 Skill 市场（SkillHub / ClawHub），可以直接使用这些平台发布的技能包。

| 平台 | 地址 | 区域 |
|------|------|------|
| **SkillHub** | [https://skillhub.cn](https://skillhub.cn/) | 中国 |
| **ClawHub** | [https://clawhub.ai](https://clawhub.ai/) | 海外 |

> **说明：** VibeCoding 不内建 Skill 市场，但采用标准的技能目录格式（`SKILL.md`），
> 与 SkillHub / ClawHub 发布的技能包完全兼容。从市场下载的技能放入技能目录即可直接使用，
> 无需任何额外适配。

本指南涵盖：

1. [从市场安装技能](#从市场安装技能) — 三步完成
2. [技能格式兼容](#技能格式兼容) — 标准格式说明
3. [本地技能系统](#本地技能系统) — 已实现的功能
4. [Cron 基础设施](#cron-基础设施) — 定时任务基础

---

## 从市场安装技能

从 SkillHub / ClawHub 安装技能只需三步：

### 1. 下载技能包

从市场下载技能包（通常是一个包含 `SKILL.md` 的目录或压缩包）。

### 2. 解压到技能目录

```bash
# 全局安装（所有项目可用）
# Linux/macOS:
unzip go-expert.zip -d ~/.vibecoding/skills/
# Windows:
Expand-Archive go-expert.zip -DestinationPath "$env:APPDATA\vibecoding\skills\"

# 项目级安装（仅当前项目可用）
unzip go-expert.zip -d .skills/
```

### 3. 验证安装

```
> /skills
Loaded 3 skills:
  - go-expert (global)        ← 刚安装的
  - coding-standards (global)
  - project-conventions (project)
```

就这么简单。技能已被自动加载并注入系统提示词。

---

## 技能格式兼容

VibeCoding 的技能格式与 SkillHub / ClawHub 标准完全一致：

```
skill-name/
├── SKILL.md              # 必需：技能定义文件
└── references/           # 可选：按需加载的参考文件
    ├── api-guide.md
    └── examples.md
```

### SKILL.md 标准格式

```markdown
# 技能名称

简短描述。

## 规则

- 规则 1
- 规则 2

## 示例

...
```

### 参考文件

技能可以包含 `references/` 目录下的参考文件，通过 `skill_ref` 工具按需加载：

```
> skill_ref(skill="go-expert", ref="references/api-guide.md")
→ 返回 api-guide.md 的内容
```

这允许技能包含大量参考资料而不占用系统提示词空间。

---

## 本地技能系统

除了从市场下载，你也可以直接创建本地技能。

### 技能目录

| 类型 | 位置 | 作用域 |
|------|------|--------|
| 全局 | `~/.vibecoding/skills/`（Linux/macOS）或 `%APPDATA%\vibecoding\skills\`（Windows） | 所有项目 |
| 项目 | `.skills/`（项目根目录） | 当前项目，覆盖同名全局技能 |

### 创建技能

```bash
mkdir -p ~/.vibecoding/skills/go-expert
cat > ~/.vibecoding/skills/go-expert/SKILL.md << 'EOF'
# Go Expert

专家级 Go 编码规范。

## 规则

- 使用 `gofmt` 格式化代码
- 遵循 Effective Go 指南
- 返回错误，不要 panic
- 使用 `fmt.Errorf` 和 `%w` 包装错误

## 测试

- 编写表驱动测试
- 使用 `t.Run` 子测试
- 目标覆盖率 >80%
EOF
```

### 使用技能

```
> /skills
已加载 2 个技能:
  - go-expert (全局)
  - project-conventions (项目)

> /skill:go-expert
已加载技能: go-expert
```

### 配置

在 `settings.json` 中配置全局技能目录：

```json
{
  "skillsDir": "~/.vibecoding/skills"
}
```

项目技能自动从 `.skills/` 加载，无需额外配置。

---

## Cron 基础设施

VibeCoding 已有内部 cron 基础设施（`internal/cron` 包）和 TUI 命令入口。Cron 存储将任务持久化到 `~/.vibecoding/cron.json`，调度器每 30 秒检查一次到期任务。

### `/cron` TUI 命令

需要多 Agent 模式（`--multi-agent` 或 Ctrl+P 切换）：

```
> /cron add <描述>             — 添加定时任务
> /cron list                   — 列出定时任务
> /cron enable <id>            — 启用任务
> /cron disable <id>           — 禁用任务
> /cron remove <id>            — 删除任务
> /cron run <id>               — 立即运行任务
```

### Cron 任务数据模型

| 字段 | 描述 |
|------|------|
| `id` | 唯一任务 ID（如 `cron-1716883200`） |
| `name` | 任务简短描述 |
| `prompt` | 发送给子 Agent 的任务提示词 |
| `schedule` | 5 字段 cron 表达式 |
| `mode` | `agent` 或 `yolo` |
| `enabled` | 任务是否激活 |
| `last_run` | 上次执行时间戳 |
| `next_run` | 计算得出的下次执行时间 |
| `run_count` | 总执行次数 |
| `last_status` | `success`、`failed` 或 `running` |

### 调度器架构

```
调度器循环 (每 30 秒)
    │
    ├── 从存储列出所有已启用任务
    │
    ├── 检查每个任务：是否到期？
    │   ├── 从未运行 → 到期
    │   ├── NextRun 已过 → 到期
    │   └── 上次运行超过 1 小时 → 到期（兜底）
    │
    └── 到期任务 → 创建子 Agent
              │
              ├── 标记任务为 "running"
              ├── 通过 AgentManager 创建 Agent
              ├── 使用任务 prompt 运行 Agent
              ├── 收集结果
              └── 更新任务状态 (success/failed)
```

---

## 相关文档

- [技能系统](skills.md) — 本地技能格式和管理
- [配置详解](configuration.md) — 完整设置参考
- [安全与沙箱](security.md) — 沙箱和审批控制
