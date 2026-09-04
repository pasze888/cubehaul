## CubeHaul v0.2.0

### 变更

- **自动重试**：429/5xx/网络抖动自动重试——共 4 次尝试、指数退避加随机抖动（750ms 起步、8s 封顶）、尊重 `Retry-After`（封顶 30s），每次重试提示到 stderr；脚本化批量使用不再一遇限流就失败
- **CurseForge 版本列表取全**：`versions` 对单值 `--loader`/`--game-version` 做服务端下推（`modLoaderType`/`gameVersion`）并按 `index` 翻页（pageSize 200、上限 1000 条并提示）——热门项目不再只见前 50 个文件
- **`--limit` 诚实化**：超过平台单页上限（Modrinth 100、CurseForge 50）时钳制并在 stderr 提示，不再静默截断；分页用 `--offset`
- **版本号单一来源**：`--version` 与默认 User-Agent 由 tag 经 `-ldflags -X` 注入，tag 即事实来源；手编包报 `dev`；配置的 `user_agent` 现在对 CDN 下载同样生效
- 文档与 403 报错移除第三方镜像示例（API base 覆盖能力保留）

### 平台

| 文件 | 平台 |
|---|---|
| `cubehaul-windows-amd64.exe` | Windows x64 |
| `cubehaul-linux-amd64` / `cubehaul-linux-arm64` | Linux |
| `cubehaul-darwin-amd64` / `cubehaul-darwin-arm64` | macOS |

## CubeHaul v0.1.0 — haul cubes from the mod mines

搜索、查看、下载 Minecraft 模组，同时支持 **Modrinth** 和 **CurseForge** 的 Go CLI 工具。

### 功能

- **search** — 双平台搜索，便捷参数（`--loader` / `--game-version` / `--category` / `--sort` 等）+ Modrinth facet 精细过滤（`--facet` / `--facets-json`）+ CurseForge `--raw-param` 原始透传
- **info** — 项目详情（作者、下载量、许可证、分类）
- **versions** — 版本列表，支持 loader/游戏版本过滤，ID 直接喂给 download
- **download** — 三种版本选择（`--version-id` / `--latest` / `--loader`+`--game-version`），`.part` 临时文件 + 大小校验
- **categories** — 分类树（CurseForge 免 key 可查）
- **系统代理自动检测** — Windows 注册表 / `CUBEHAUL_PROXY` / 标准环境变量，含绕过规则
- `--json` 输出、分组帮助文档、Examples

### 快速开始

```bash
cubehaul modrinth search sodium --loader fabric --limit 5
cubehaul modrinth download sodium --latest --output-dir ./mods
```

### 平台

| 文件 | 平台 |
|---|---|
| `cubehaul-windows-amd64.exe` | Windows x64 |
| `cubehaul-linux-amd64` / `cubehaul-linux-arm64` | Linux |
| `cubehaul-darwin-amd64` / `cubehaul-darwin-arm64` | macOS |

### 说明

- Modrinth 免认证（自动携带 User-Agent）；CurseForge 需 API key：`CURSEFORGE_API_KEY` 环境变量或 `~/.cubehaul/config.json`
- 源码：https://github.com/pasze888/cubehaul
