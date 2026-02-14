# 每日打卡系统文档

## 概述

这是一个基于 Gin 框架的每日打卡系统, 是作者用来提醒自己和好伙伴每天刷力扣的工具。

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

打卡凭证文件存储在 `uploads` 目录下，每日结算完成会进行清空以免占用服务器资源，按日期分目录存储：

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

## 启动服务

```bash
go run main.go
```

服务将在 `http://localhost:8080` 启动。

默认配置访问路径为`http://localhost:8080?token=12138`


