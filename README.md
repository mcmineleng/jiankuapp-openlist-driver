# 简库资源网 OpenList 驱动

## 简介

本驱动基于 **预生成 JSON + 本地/HTTP 远程存储 JSON** 的方式工作。

> ⚠️ 在爬取「简库资源网」之前，请务必先征得站长 **墨影XZ** 的同意。

---

## 一、生成 JSON

### 方式一：直接使用现成文件

```
https://assets.mcleng.cn/jiankuapp/dir.json
```

### 方式二：自行生成

#### 1. 准备工具

需要准备两个二进制文件：

- `dir-json`（或 `dir-json.exe`）
- `full-json`（或 `full-json.exe`）

获取方式二选一：

- 直接下载现成的二进制
- 自行编译（见下文「编译工具」）

#### 2. 生成 `full.json`

**Linux / BSD / macOS**

```shell
./full-json --all > full.json
```

**Windows**

```cmd
.\full-json.exe --all > full.json
```

#### 3. 转换为 `dir.json`

**Linux / BSD / macOS**

```shell
cat full.json | ./dir-json > dir.json
```

**Windows**

```cmd
type full.json | .\dir-json.exe > dir.json
```

---

## 二、编译工具
需要Go 1.16+和UPX(非必须，可减小程序体积)
进入 `utils` 目录，运行：

```shell
./build.sh
```

即可得到 `full-json` 和 `dir-json` 两个二进制文件。

---

## 三、编译 OpenList

### 1. 拉取官方仓库

```shell
git clone https://github.com/OpenListTeam/OpenList
```

### 2. 下载前端

从以下地址下载前端发行包：

```
https://github.com/OpenListTeam/OpenList-Frontend/releases
```

解压至 OpenList 仓库的 `public/dist` 目录。

### 3. 集成驱动

将本仓库的 `drivers` 目录复制到 OpenList 仓库目录下（需要覆盖原有的 `all.go`）。

### 4. 编译

在 OpenList 仓库根目录运行：

```shell
go build -ldflags="-w -s" -tags=jsoniter .
```

编译完成后得到 `OpenList`（Windows 下为 `OpenList.exe`），按官方方式启动即可。

---

## 四、添加存储

1. 在「添加存储」页面选择 **JiankuAPP** 驱动。
2. 向下滚动找到 **App tree url** 字段。
3. 填入以下任一地址：

   - **远程地址（默认）**：

     ```
     https://assets.mcleng.cn/jiankuapp/dir.json
     ```

   - **本地文件路径**：按前文方法自行生成 `dir.json` 后，填写其本地路径（加载更快）。

---
