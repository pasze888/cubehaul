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
cubehaul search sodium --platform modrinth --loader fabric --limit 5
cubehaul download modrinth sodium --latest --output-dir ./mods
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
