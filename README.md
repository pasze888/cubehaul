# ModFetch

搜索、查看、下载 Minecraft 模组 —— 同时支持 **Modrinth** 和 **CurseForge** 的 Go CLI 工具。

## 构建

```bash
go build -o modfetch .
```

## 命令

```bash
# 搜索（--platform 必填）
modfetch search sodium --platform modrinth --loader fabric --game-version 1.20.1 --limit 10
modfetch search "" --platform modrinth --category adventure --sort downloads
modfetch search --platform curseforge --loader forge --game-version 1.20.1 --category technology

# 项目详情
modfetch info modrinth sodium
modfetch info curseforge 394468

# 版本列表（ID 列可直接用于 download --version-id）
modfetch versions modrinth sodium --loader fabric --game-version 1.20.1
modfetch versions curseforge 394468 --loader forge

# 下载（三种版本选择方式）
modfetch download modrinth sodium --latest
modfetch download modrinth sodium --loader fabric --game-version 1.20.1
modfetch download curseforge 394468 --version-id 5730579 --output-dir ./mods

# 分类列表（CurseForge 无需 API key）
modfetch categories curseforge --class-id 6
modfetch categories modrinth
```

所有输出命令均支持 `--json`。

## 搜索过滤

### 通用便捷参数

| 参数 | 说明 |
|---|---|
| `--project-type` | mod / modpack / resourcepack / shader / plugin / datapack / ...（CF 映射为 classId） |
| `--category` | 分类名/slug，可重复（同组 OR）；CF 自动按名称查分类表转 ID |
| `--loader` | fabric / forge / neoforge / quilt / ...，可重复 |
| `--game-version` | 如 1.20.1，可重复 |
| `--sort` | modrinth: relevance/downloads/follows/newest/updated；curseforge: featured/popularity/updated/name/author/downloads |
| `--limit` / `--offset` | 分页（CF 单页上限 50） |

### Modrinth 专有（facet 精细控制）

Modrinth 的 facet 语法：内层数组 OR、外层 AND，`:`/`=` 表示等于，`!=` `>=` `<=` `>` `<` 紧跟类型后做比较（注意比较操作符**不带冒号**）。

```bash
# 便捷参数自动展开为 facet 组
modfetch search "" --platform modrinth --category adventure --category technology --loader fabric --game-version 1.20.1

# 原始 facet 透传，可重复
modfetch search "" --platform modrinth --facet 'downloads>=100000000' --facet 'versions!=1.20.1'

# 原始 JSON facet 组（追加 AND 组，优先级最高）
modfetch search "" --platform modrinth --facets-json '[["categories:forge"],["versions:1.17.1"]]'

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

> 平台不支持的参数会在请求前报错（如 modrinth 的 `--facet` 用于 curseforge、`--author` 用于 curseforge）。

## 系统代理

工具自动使用系统代理，无需配置（API 请求和文件下载都生效）：

1. `MODFETCH_PROXY` 环境变量（显式覆盖，优先级最高），如 `http://127.0.0.1:7897` 或 `socks5://127.0.0.1:7890`
2. 标准 `HTTP_PROXY` / `HTTPS_PROXY` / `NO_PROXY` 环境变量
3. **Windows 系统代理**（Internet 选项里的代理设置，读取注册表 `ProxyEnable`/`ProxyServer`/`ProxyOverride`，含 `localhost`、`127.*`、`<local>` 等绕过规则）
4. 都没有时直连

设 `MODFETCH_DEBUG=1` 可在 stderr 打印实际使用的代理。macOS/Linux 没有注册表代理，请用环境变量方式。

## 配置

```bash
# 方式一：环境变量
export CURSEFORGE_API_KEY=xxxxxxxx

# 方式二：配置文件 ~/.modfetch/config.json
{
  "curseforge_api_key": "xxxxxxxx",
  "user_agent": "myname/1.0 (me@example.com)"
}
```

- CurseForge 的搜索/详情/版本接口需要 API key（[申请地址](https://console.curseforge.com)）；`categories` 可匿名访问。
- Modrinth 无需认证，但强制携带 User-Agent（默认 `modfetch/0.1.0`，可在配置里替换为你的联系方式）。
- 没有 key 时给出明确报错提示。

## 实现要点

- `internal/platform`：两个平台的客户端 + 统一 `Platform` 接口（Search/GetProject/ListVersions/Categories），`SearchOptions` 承载全部分支参数，各平台 `BuildQuery` 自行映射
- CurseForge 文件 `downloadUrl` 为空时按 `fileId/1000 / fileId%1000` 拼 ForgeCDN 链接
- 下载走 `.part` 临时文件 + 大小校验，失败自动清理
- 限流注意：Modrinth 300 req/min，CurseForge 50 req/10s
