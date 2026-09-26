# Changelog

## v0.3.0 — 2026-09-26

### 新增

- `config` 子命令组：`list` / `get` / `set` / `unset` / `path`，读写 `~/.cubehaul/config.json`，取代手改 JSON
  - `list` 显示每个键的**有效值**与来源（`env` / `file` / `default` / `unset`），`curseforge_api_key` 默认掩码，`--show-secrets` 显示原文
  - `set` 先校验（URL 形态、非空、无控制字符）再原子替换文件，保留手写的未知键；POSIX 下文件权限 0600
  - `get` 逐字输出单个键的有效值供脚本取用；`unset` 删除条目回退默认；`path` 打印配置文件位置
- `--json` 支持字段裁剪：`--json=field,field`（裸 `--json` 仍输出全字段）；字段契约与顺序见 [docs/json-fields.md](docs/json-fields.md)
- `--jq <表达式>`：内置 gojq 过滤 JSON 输出，无需外部 `jq`（新增依赖 github.com/itchyny/gojq，MIT）
- `download` 支持 `--json`，输出 `path`/`size` 等结果对象（进度条固定在 stderr，不污染 stdout）
- 新增 `api <path>` 兜底命令：任意只读 GET 端点，`-f key=value` 传查询参数
- `view` 成为项目详情的正式命令名，`info` 保留为别名
- `--web` / `-w`：`search` / `view` / `versions` 用浏览器打开对应页面
- `--debug` 全局 flag：等价于 `CUBEHAUL_DEBUG=1`，并额外打印代理选择结果
- 新增 `target [path]` 命令与 `--gradle`：从工程的 `gradle.properties` 识别 Minecraft 版本与加载器
  - `target` 只读报告识别结果（默认从当前目录向上找，最近的文件没声明目标时继续向上，支持 `--json` 字段裁剪）
  - `search` / `versions` / `download` 的 `--gradle` 把识别结果补进 `--game-version` / `--loader`；显式 flag 优先，多加载器工程不猜，所有取舍打到 stderr
  - 属性名按真实 MDK 归一化后扫描全文件：大小写/下划线/点号变体、工具链/target 前缀键（`neoforge_121_minecraft_version`）、以及只声明版本区间时取区间下界
  - `mod_loader` / `modLoaders` 显式声明优先于键名推断；依赖项的版本键（如 `emiMinecraftVersion`）不会被误当成工程目标

### 变更

- 文档里 `search ""` 的冗余写法去掉——查询词本来就是可选的
- 输出渲染层改为返回错误：`--json` 字段名写错会立刻报错并列出合法字段

## v0.2.0 — 2026-09-04

### 新增

- 自动重试：429/5xx/网络抖动自动重试（最多 4 次尝试、指数退避加随机抖动、尊重 `Retry-After`，过程提示到 stderr）
- CurseForge `versions` 全量可见：单值 `--loader`/`--game-version` 下推为服务端过滤，按页翻页取全（上限 1000 条并提示）；多值过滤回退客户端过滤

### 变更

- `--limit` 超过平台单页上限（Modrinth 100、CurseForge 50）时钳制并在 stderr 提示，不再静默截断
- 版本号单一来源：`--version` 与默认 User-Agent 由 release tag 构建注入，tag 即事实来源；手编包报 `dev`
- 配置的 `user_agent` 现在对文件下载同样生效
- 文档与 403 报错移除第三方镜像示例（API base 覆盖能力保留）

## v0.1.0 — 2026-08-12

首个发布。

### 新增

- search：双平台搜索，便捷参数（`--loader`/`--game-version`/`--category`/`--sort` 等）+ Modrinth facet 精细过滤（`--facet`/`--facets-json`）+ CurseForge `--raw-param` 原始透传
- info / versions / download / categories：项目详情、版本列表（ID 直接喂 `download --version-id`）、三种版本选择下载、分类树（CurseForge 免 key 可查）
- 下载默认保存到系统下载文件夹（Windows `FOLDERID_Downloads` / Linux `XDG_DOWNLOAD_DIR`），带进度显示、大小校验，失败自动清理
- 系统代理自动检测：Windows 注册表（含绕过规则）/ `CUBEHAUL_PROXY` / 标准环境变量
- `--json` 输出、分组帮助文档
