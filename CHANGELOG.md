# Changelog

## v0.2.0 — 2026-09-04

### 新增

- 自动重试：429/5xx/网络抖动自动重试（最多 4 次尝试、指数退避加随机抖动、尊重 `Retry-After`，过程提示到 stderr）
- CurseForge `versions` 全量可见：单值 `--loader`/`--game-version` 下推为服务端过滤，按页翻页取全（上限 1000 条并提示）；多值过滤回退客户端过滤
- `--limit` 超过平台单页上限（Modrinth 100、CurseForge 50）时钳制并在 stderr 提示，不再静默截断

### 变更

- 版本号单一来源：`--version` 与默认 User-Agent 由 release tag 构建注入，tag 即事实来源；手编包报 `dev`
- 配置的 `user_agent` 现在对文件下载同样生效
- 文档与 403 报错移除第三方镜像示例（API base 覆盖能力保留）

## v0.1.0 — 2026-08-12

首个发布。

- search：双平台搜索，便捷参数（`--loader`/`--game-version`/`--category`/`--sort` 等）+ Modrinth facet 精细过滤（`--facet`/`--facets-json`）+ CurseForge `--raw-param` 原始透传
- info / versions / download / categories：项目详情、版本列表（ID 直接喂 `download --version-id`）、三种版本选择下载、分类树（CurseForge 免 key 可查）
- 下载默认保存到系统下载文件夹（Windows `FOLDERID_Downloads` / Linux `XDG_DOWNLOAD_DIR`），带进度显示、大小校验，失败自动清理
- 系统代理自动检测：Windows 注册表（含绕过规则）/ `CUBEHAUL_PROXY` / 标准环境变量
- `--json` 输出、分组帮助文档
