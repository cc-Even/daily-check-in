# 每日打卡系统文档

## 概述

这是一个基于 Gin 框架的每日打卡系统。

## 基础信息

- **Base URL**: `http://localhost:8989`
- **认证方式**: Token 验证（所有接口都需要）

## 认证说明

每个 API 请求都需要携带 `token` 参数进行验证。系统会计算 token 的 16 位小写 MD5 值，与配置文件中的 `tokenMd5` 进行比对。

### Token 传递方式（二选一）

1. **URL 查询参数**: `?token=your_token`
2. **请求头**: `X-Token: your_token`

### 认证失败响应

```json
{
  "code": 401,
  "message": "缺少token参数"
}
```

或

```json
{
  "code": 401,
  "message": "token验证失败"
}
```

---

## API 接口

### 1. 获取打卡人员列表

获取配置文件中的所有打卡人员信息。

**请求**

```
GET /api/persons?token={your_token}
```

**请求参数**

| 参数名 | 位置 | 类型 | 必填 | 说明 |
|--------|------|------|------|------|
| token | query/header | string | 是 | 认证令牌 |

**响应示例**

```json
{
  "code": 200,
  "message": "success",
  "data": [
    {
      "name": "Alice Johnson",
      "email": "alice@example.com",
      "avatar": "avatar/dog.png"
    },
    {
      "name": "Bob Smith",
      "email": "bob@example.com",
      "avatar": "avatar/cat.png"
    }
  ]
}
```

---

### 2. 上传打卡凭证

上传用户的打卡凭证图片。同一用户当天可以重复上传，新上传的文件会覆盖之前的记录。

**请求**

```
POST /api/upload?token={your_token}
Content-Type: multipart/form-data
```

**请求参数**

| 参数名 | 位置 | 类型 | 必填 | 说明 |
|--------|------|------|------|------|
| token | query/header | string | 是 | 认证令牌 |
| name | form-data | string | 是 | 打卡人员姓名（必须在 checkInPersonList 中） |
| image | form-data | file | 是 | 打卡凭证图片文件 |

**成功响应**

```json
{
  "code": 200,
  "message": "上传成功",
  "data": {
    "name": "Alice Johnson",
    "date": "2026-02-05",
    "filePath": "uploads/2026-02-05/Alice_Johnson.jpg"
  }
}
```

**错误响应**

缺少 name 参数：
```json
{
  "code": 400,
  "message": "缺少name参数"
}
```

name 不在打卡人员列表中：
```json
{
  "code": 400,
  "message": "name不在打卡人员列表中"
}
```

未上传图片文件：
```json
{
  "code": 400,
  "message": "请上传图片文件"
}
```

文件类型错误：
```json
{
  "code": 400,
  "message": "只支持上传图片文件"
}
```

---

### 3. 获取打卡状态

查询指定日期所有人员的打卡状态。

**请求**

```
GET /api/status?token={your_token}&date={date}
```

**请求参数**

| 参数名 | 位置 | 类型 | 必填 | 说明 |
|--------|------|------|------|------|
| token | query/header | string | 是 | 认证令牌 |
| date | query | string | 否 | 查询日期，格式：YYYY-MM-DD，默认为当天 |

**响应示例**

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "date": "2026-02-05",
    "status": [
      {
        "name": "Alice Johnson",
        "avatar": "avatar/dog.png",
        "uploaded": true
      },
      {
        "name": "Bob Smith",
        "avatar": "avatar/cat.png",
        "uploaded": false
      }
    ]
  }
}
```

---

## 定时任务

### 提醒任务 (mentionTime)

- **触发时间**: 配置文件中的 `mentionTime`（如 "21:00"）
- **功能**: 检查当天未上传打卡凭证的用户，给每个未完成的用户单独发送提醒邮件
- **邮件主题**: "打卡提醒"
- **邮件内容**: 提醒用户尽快完成打卡

### 打卡检查任务 (checkInTime)

- **触发时间**: 配置文件中的 `checkInTime`（如 "23:00"）
- **功能**: 检查当天未上传打卡凭证的用户，群发邮件给所有人
- **邮件主题**: "每日打卡未完成人员通知"
- **邮件内容**: 包含所有未完成打卡人员的姓名和邮箱

---

## 配置文件说明

配置文件为 `config.json`，位于项目根目录。

```json
{
  "checkInPersonList": [
    {
      "name": "Alice Johnson",
      "email": "alice@example.com"
    },
    {
      "name": "Bob Smith",
      "email": "bob@example.com"
    }
  ],
  "mentionTime": "21:00",
  "checkInTime": "23:00",
  "tokenMd5": "your_token_md5_here",
  "smtpHost": "smtp.gmail.com",
  "smtpPort": 587,
  "smtpUser": "your_email@gmail.com",
  "smtpPassword": "your_app_password",
  "fromEmail": "your_email@gmail.com"
}
```

### 配置项说明

| 字段名 | 类型 | 说明 |
|--------|------|------|
| checkInPersonList | array | 打卡人员列表 |
| checkInPersonList[].name | string | 人员姓名 |
| checkInPersonList[].email | string | 人员邮箱 |
| mentionTime | string | 提醒时间（格式：HH:MM） |
| checkInTime | string | 打卡检查时间（格式：HH:MM） |
| tokenMd5 | string | Token 的 16 位小写 MD5 值（取中间 16 位） |
| smtpHost | string | SMTP 服务器地址 |
| smtpPort | int | SMTP 服务器端口 |
| smtpUser | string | SMTP 认证用户名 |
| smtpPassword | string | SMTP 认证密码 |
| fromEmail | string | 发件人邮箱地址 |

---

## 文件存储

打卡凭证文件存储在 `uploads` 目录下，按日期分目录存储：

```
uploads/
├── 2026-02-05/
│   ├── Alice_Johnson.jpg
│   └── Bob_Smith.png
├── 2026-02-06/
│   ├── Alice_Johnson.jpg
│   └── Bob_Smith.jpg
└── ...
```

---

## 使用示例

### 使用 cURL 获取人员列表

```bash
curl "http://localhost:8080/api/persons?token=your_secret_token"
```

### 使用 cURL 上传打卡凭证

```bash
curl -X POST "http://localhost:8080/api/upload?token=your_secret_token" \
  -F "name=Alice Johnson" \
  -F "image=@/path/to/your/image.jpg"
```

### 使用 cURL 获取打卡状态

```bash
# 获取今天的打卡状态
curl "http://localhost:8080/api/status?token=your_secret_token"

# 获取指定日期的打卡状态
curl "http://localhost:8080/api/status?token=your_secret_token&date=2026-02-05"
```

---

## 错误码说明

| 状态码 | 说明 |
|--------|------|
| 200 | 请求成功 |
| 400 | 请求参数错误 |
| 401 | 认证失败 |
| 500 | 服务器内部错误 |

---

## 启动服务

```bash
go run main.go
```

服务将在 `http://localhost:8080` 启动。

默认配置访问路径为`http://localhost:8080?token=12138`


