# CubeHaul

> haul cubes from the mod mines — 搜索、查看、下载 Minecraft 模组，同时支持 **Modrinth** 和 **CurseForge** 的 Go CLI 工具。

## 构建

```bash
go build -o cubehaul .
```

手编包的 `cubehaul --version` 报 `dev`；正式版本号由 release tag 构建时注入。

## 命令

平台相关的命令都挂在平台子命令下（`cubehaul modrinth ...` / `cubehaul curseforge ...`，各自只暴露本平台支持的 flag）；另有与平台无关的 `cubehaul config`。两个平台子命令都有缩写：`modrinth` → `mr`、`curseforge` → `cf`：

```bash
# 搜索（query 可选，省略则按过滤条件列出）
cubehaul modrinth search sodium --loader fabric --game-version 1.20.1 --limit 10
cubehaul modrinth search --category adventure --sort downloads
cubehaul curseforge search sodium --loader forge --game-version 1.20.1 --category technology

# 项目详情（view；info 是它的别名）
cubehaul modrinth view sodium
cubehaul curseforge view 394468
cubehaul modrinth view sodium --web        # 改用浏览器打开项目页

# 版本列表（ID 列可直接用于 download --version-id）
cubehaul modrinth versions sodium --loader fabric --game-version 1.20.1
cubehaul curseforge versions 394468 --loader forge

# 下载（三种版本选择方式）
cubehaul modrinth download sodium --latest
cubehaul modrinth download sodium --loader fabric --game-version 1.20.1
cubehaul curseforge download 394468 --version-id 5730579 --output-dir ./mods

# 分类列表（CurseForge 无需 API key）
cubehaul curseforge categories --class-id 6
cubehaul modrinth categories

# 平台 API 兜底：任意只读端点，-f 传查询参数
cubehaul modrinth api /project/sodium --jq .downloads
cubehaul curseforge api /mods/394468 --jq .data.downloadCount

# 用缩写（与上面完全等价）
cubehaul mr search sodium --loader fabric
cubehaul cf view 394468
```

### 输出

默认打印表格；`--json` 打印 JSON，`--json=field,field` 只保留指定字段，`--jq '<表达式>'` 用 jq 语法过滤（内置 gojq，**不需要装 `jq`**；`--jq` 隐含 `--json`）：

```bash
cubehaul mr search sodium --json
cubehaul mr search sodium --json=id,title,downloads
cubehaul mr search sodium --jq 'sort_by(.downloads) | reverse | .[0:5] | .[].title'
cubehaul mr versions sodium --json=id,version_number,loaders
```

> `--json` 的取值必须用 `=`：裸 `--json` 就是“全部字段”，写成 `--json id,title` 时
> `id,title` 会被当成位置参数。字段名与输出顺序见 [docs/json-fields.md](docs/json-fields.md)。

`--debug` 把请求、重试与代理诊断打到 stderr（等价于 `CUBEHAUL_DEBUG=1`）；结果输出始终只走 stdout，进度条也固定在 stderr。

> `download` 默认保存到**系统下载文件夹**（Windows 取 `FOLDERID_Downloads`，兼容 OneDrive 重定向；Linux 读 `XDG_DOWNLOAD_DIR`，回退 `~/Downloads`），目录不存在会自动创建。解析不到时直接报错，需显式传 `--output-dir`。它**不会**自动寻找某个 Minecraft 实例的 mods 目录，要直接落到实例里请显式指定 `--output-dir`。

### mod 工程目标（gradle.properties）

在 mod 工程目录里加 `--gradle`，就会从 `gradle.properties` 读出工程目标的 **Minecraft 版本**与**加载器**，补进 `--game-version` / `--loader`：

```bash
cd MyMod
cubehaul target                            # 先看识别到什么（只读，不发请求）
cubehaul mr download sodium --latest --gradle
cubehaul mr search jei --gradle            # 只找你这个目标下的版本
cubehaul mr versions sodium --gradle
```

- 裸 `--gradle` 从当前目录**向上**找 `gradle.properties`；最近这份没写版本/加载器时会继续往上（多模块工程常把目标写在根文件里），真正给出答案的文件会打在 stderr，也可以用 `target` 的 `FILE` 看。`--gradle=<文件|目录>` 从指定位置开始（直接给文件就只用该文件）。
- 显式写了 `--game-version` / `--loader` 时以你写的为准，被忽略的值会在 stderr 说明。
- 识别到多个加载器（Architectury 这类多加载器工程）时**不猜**，提示你显式传 `--loader`。
- 属性名不是固定列表：`minecraft_version`、`mcVersion`、`minecraftVersion`、`neoforge_121_minecraft_version` 这类 target 前缀键都能认；工程只写了 `minecraft_version_range` 时取区间下界，并用 `VERSION SOURCE` 标出它的来源，方便复核。
- `mod_loader` / `modLoaders` 这类**显式声明优先**于键名推断（有些工程把 NeoForge 的版本写成 `forgeVersion`）；依赖项的版本键（如 `emiMinecraftVersion`）不会被当成工程目标。
- `--version-id` 已经点明确切版本，此时 `--gradle` 不参与。

### 配置命令

`cubehaul config` 直接读写 `~/.cubehaul/config.json`，不必手改 JSON：

```bash
cubehaul config list                       # 每个键的有效值 + 来源；密钥默认掩码
cubehaul config list --show-secrets        # 显示密钥原文
cubehaul config get curseforge_api_key     # 只打印有效值，供脚本取用
cubehaul config set curseforge_api_key "$CF_API_KEY"
cubehaul config set user_agent "myname/1.0 (me@example.com)"
cubehaul config unset curseforge_api_base  # 删掉该键，回退内置默认
cubehaul config path                       # 配置文件位置
```

有效值优先级是 **环境变量 > 配置文件 > 内置默认**，`config list` 的 `SOURCE` 列显示实际生效的来源（`env` / `file` / `default` / `unset`）。`set` 会先校验（URL 形态、非空、无控制字符）再**原子替换**文件，手写在同一文件里的其它键不会丢；POSIX 下文件权限 0600（Windows 忽略该位）。`get` 面向脚本逐字输出，因此不掩码；只有 `list` 掩码。

## 搜索过滤

### 通用便捷参数

| 参数 | 说明 |
|---|---|
| `--project-type` | mod / modpack / resourcepack / shader / plugin / datapack / ...（CF 映射为 classId） |
| `--category` | 分类名/slug，可重复（同组 OR）；CF 自动按名称查分类表转 ID |
| `--loader` | fabric / forge / neoforge / quilt / ...，可重复 |
| `--game-version` | 如 1.20.1，可重复 |
| `--sort` | modrinth: relevance/downloads/follows/newest/updated；curseforge: relevancy/featured/popularity/updated/name/author/downloads/category/game-version |
| `--limit` / `--offset` | 分页（单页上限：modrinth 100、curseforge 50；超限会钳制并在 stderr 提示，用 `--offset` 翻页） |

> **缺省排序**：带查询词时两个平台都按**相关度**排（Modrinth 的 `relevance`、CurseForge 的 `relevancy`/`sortField=13`）。CF 的纯过滤搜索（query 为空，如 `search --category technology`）不发 sortField，用服务端默认序——空查询下相关度无定义。
>
> CF 的排序方向总会显式发出：`popularity`/`updated`/`downloads`/`relevancy` 走 `desc`，`name`/`author` 走 `asc`，`featured`/`category`/`game-version` 不指定（沿用服务端顺序）。`--sort-order` 可覆盖。

> **CurseForge `versions` 的过滤下推**：`--loader`/`--game-version` 只给**一个**值（且 loader 名字能对应到平台枚举）时，该值作为服务端过滤条件下发；多值或无法对应的值回退为本地过滤——先按其余可下推的条件翻页取回文件列表，再在本地筛（下发过的条件也会本地复查），因此请求页数会更多。无论走哪条路径，单次最多取回 1000 个文件，命中上限会在 stderr 提示收窄过滤条件。

### Modrinth 专有（facet 精细控制）

Modrinth 的 facet 语法：内层数组 OR、外层 AND，`:`/`=` 表示等于，`!=` `>=` `<=` `>` `<` 紧跟类型后做比较（注意比较操作符**不带冒号**）。

```bash
# 便捷参数自动展开为 facet 组
cubehaul modrinth search --category adventure --category technology --loader fabric --game-version 1.20.1

# 原始 facet 透传，可重复
cubehaul modrinth search --facet 'downloads>=100000000' --facet 'versions!=1.20.1'

# 原始 JSON facet 组（追加 AND 组，优先级最高）
cubehaul modrinth search --facets-json '[["categories:forge"],["versions:1.17.1"]]'

# 其它便捷过滤（仅 modrinth）
--open-source / --no-open-source   # 开源过滤
--environment client|server|client_and_server
--author <用户名>   --license <SPDX id，如 mit>
```

### CurseForge 专有

```bash
--sort-order asc|desc          # 排序方向
--class-id <n>                 # 手动指定 classId（覆盖映射，默认 6）
--category-id <n>              # 直接传分类 ID
--mod-id <n>                   # 按 Mod ID 精确获取（直接查 /mods/{id}）
--slug <name>                  # 按 slug 搜索
--game-version-type-id <n>     # 1=Release 2=Beta 3=Alpha
--raw-param 'key=value'        # 任意查询参数原样透传，可重复
```

> 平台不支持的 flag 不会出现在对应子命令上（如 `--facet`、`--author` 只属于 `modrinth search`，`--raw-param` 只属于 `curseforge search`）。

## 系统代理

工具自动使用系统代理，无需配置（API 请求和文件下载都生效）：

1. `CUBEHAUL_PROXY` 环境变量（显式覆盖，优先级最高），如 `http://127.0.0.1:7897` 或 `socks5://127.0.0.1:7890`
2. 标准 `HTTP_PROXY` / `HTTPS_PROXY` / `NO_PROXY` 环境变量
3. **Windows 系统代理**（Internet 选项里的代理设置，读取注册表 `ProxyEnable`/`ProxyServer`/`ProxyOverride`，含 `localhost`、`127.*`、`<local>` 等绕过规则）
4. 都没有时直连

设 `CUBEHAUL_DEBUG=1` 可在 stderr 打印实际使用的代理。macOS/Linux 没有注册表代理，请用环境变量方式。

## 配置

```bash
# 方式一：环境变量
export CURSEFORGE_API_KEY=xxxxxxxx

# 方式二：配置文件 ~/.cubehaul/config.json
{
  "curseforge_api_key": "xxxxxxxx",
  "user_agent": "myname/1.0 (me@example.com)"
}
```

方式三：用 `cubehaul config set <key> <value>` 写入，先校验再原子替换文件（见上文「配置命令」）。

默认 base 为官方 `https://api.curseforge.com/v1` 与 `https://api.modrinth.com/v2`，可用 `CURSEFORGE_API_BASE` / `MODRINTH_API_BASE`（或配置文件里的 `curseforge_api_base` / `modrinth_api_base` 字段）覆盖。

- CurseForge 的官方接口搜索/详情/版本需要 API key（[申请地址](https://console.curseforge.com)）；`categories` 可匿名访问。没有 key 时工具会明确报错并给出解决方式。
- Modrinth 无需认证，但强制携带 User-Agent（默认 `cubehaul/<版本>`，可在配置里替换为你的联系方式）。

## 限流与重试

API 请求和文件下载对限流（429）、服务端临时错误（5xx）和网络抖动会自动重试（覆盖 API 请求与下载的**建连阶段**，最多 4 次尝试，尊重 `Retry-After`，过程提示到 stderr）。限流额度：Modrinth 300 req/min，CurseForge 50 req/10s。下载一旦开始传输，中途断开不会续传也不会重试，重跑命令即可（`.part` 会从头重写）。
