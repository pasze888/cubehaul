# JSON 字段表

所有输出命令默认打印表格；`--json` 打印 JSON，`--json=field,field` 只保留指定字段。

> **取值必须用 `=`**。裸 `--json` 表示“全部字段”（该 flag 带可选值），所以写 `--json=id,title`；
> 写成 `--json id,title` 时 `id,title` 会被当成位置参数——在 `search` 里就是搜索词。

字段名即下表的键名，输出顺序也与表中一致。`--jq '<表达式>'` 在 JSON 之上做过滤，语义同 `jq`
（内置 [gojq](https://github.com/itchyny/gojq)，**无需安装 `jq`**）；`--jq` 隐含 `--json`。

| 命令 | 资源 | 字段（按输出顺序） |
|---|---|---|
| `modrinth search` / `curseforge search` | 项目 | `platform` `id` `slug` `title` `description` `author` `downloads` `follows` `categories` `license` `url` `updated_at` |
| `view`（别名 `info`） | 项目 | 同上 |
| `versions` | 版本 | `id` `project_id` `name` `version_number` `date_published` `game_versions` `loaders` `files` `changelog` |
| `categories` | 分类 | `id` `name` `slug` `class_id` `parent_id` `is_class` |
| `download` | 下载结果 | `platform` `project` `version` `version_id` `file` `url` `path` `size` |
| `config list` / `config get` | 配置项 | `key` `value` `source` `secret` |
| `target` | 工程目标 | `file` `minecraft_version` `minecraft_version_source` `loaders` |
| `api <path>` | 平台原始响应 | 由远端决定；不支持 `--json=字段`，请用 `--jq` |

字段列表的权威定义在 [`internal/output/fields.go`](../internal/output/fields.go)，
`TestFieldListsMatchStructs` 保证它与结构体同步，不会悄悄漂移。

## 项目（search / view）

| 字段 | 类型 | 说明 |
|---|---|---|
| `platform` | string | `modrinth` 或 `curseforge` |
| `id` | string | 平台 ID；CurseForge 是数字 ID 的字符串形式 |
| `slug` | string | 平台内唯一短名 |
| `title` | string | 项目名 |
| `description` | string | 简介；搜索接口有时不返回（空串） |
| `author` | string | 主要作者；CurseForge 取作者列表第一个 |
| `downloads` | number | 总下载量 |
| `follows` | number | Modrinth 是关注数，CurseForge 是点赞数 |
| `categories` | string[] | 分类名列表 |
| `license` | string | 许可证；仅 Modrinth 提供 |
| `url` | string | 项目主页 |
| `updated_at` | string | 最后更新时间（RFC 3339） |

## 版本（versions）

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 版本 ID，直接喂给 `download --version-id` |
| `project_id` | string | 所属项目 |
| `name` | string | 版本名 |
| `version_number` | string | 版本号 |
| `date_published` | string | 发布时间（RFC 3339） |
| `game_versions` | string[] | 支持的 Minecraft 版本 |
| `loaders` | string[] | 支持的加载器 |
| `files` | object[] | 文件列表，元素为 `{id,filename,url,size,primary}`；只能整体取出，不能只选子字段 |
| `changelog` | string | 更新日志，可能很长 |

## 分类（categories）

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 分类 ID |
| `name` | string | 名称，可直接用于 `search --category` |
| `slug` | string | slug |
| `class_id` | number | 所属 class；仅 CurseForge，且为 0 时不出现 |
| `parent_id` | number | 父分类；仅 CurseForge，且为 0 时不出现 |
| `is_class` | bool | 是否为 class 根节点；仅 CurseForge，为 false 时不出现 |

## 下载结果（download）

| 字段 | 类型 | 说明 |
|---|---|---|
| `platform` | string | 平台 |
| `project` | string | 命令里给的 id/slug |
| `version` | string | 版本号 |
| `version_id` | string | 版本 ID |
| `file` | string | 落盘文件名 |
| `url` | string | 实际下载的 CDN 地址 |
| `path` | string | 落盘的绝对路径 |
| `size` | number | 字节数 |

## 配置项（config list / config get）

| 字段 | 类型 | 说明 |
|---|---|---|
| `key` | string | 配置键名 |
| `value` | string | 有效值；`config list` 把密钥掩码成 `****xxxx`（`--show-secrets` 显示原文），`config get` 不掩码 |
| `source` | string | `env` / `file` / `default` / `unset` |
| `secret` | bool | 是否为密钥（即 `config list` 默认掩码）；非密钥时该字段不出现 |

## 工程目标（target）

| 字段 | 类型 | 说明 |
|---|---|---|
| `file` | string | 读到值的 gradle.properties 绝对路径；最近的这份没声明目标时会继续向上找，该字段始终指向真正给出答案的文件 |
| `minecraft_version` | string | 识别到的 Minecraft 版本；识别不到时为空串 |
| `minecraft_version_source` | string | 版本取自哪个属性；当工程只声明区间（`minecraft_version_range`）时，值由下界推断，这个字段就是给你复核用的 |
| `loaders` | string[] | 识别到的加载器，已排序；多加载器工程会多于一个，识别不到时为空数组 |

## 示例

```bash
# 只要三个字段，输出顺序即参数顺序
cubehaul modrinth search sodium --json=id,title,downloads

# 按下载量取前五
cubehaul modrinth search sodium --jq 'sort_by(.downloads) | reverse | .[0:5] | .[].title'

# 版本号 + 加载器
cubehaul modrinth versions sodium --jq '.[] | "\(.version_number) \(.loaders | join(","))"'

# 配置：只看哪些键来自环境变量
cubehaul config list --json=key,source --jq '.[] | select(.source == "env") | .key'

# 工程目标
cubehaul target --json=file,minecraft_version,loaders
```

`--jq` 的输出与 `gh ... --jq` 一致：字符串原样打印，其余按 JSON 打印，每个结果一行。
