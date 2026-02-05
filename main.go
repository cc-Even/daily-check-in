package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

// Config 配置结构体
type Config struct {
	CheckInPersonList []Person `json:"checkInPersonList"`
	MentionTime       string   `json:"mentionTime"`
	CheckInTime       string   `json:"checkInTime"`
	TokenMd5          string   `json:"tokenMd5"`
	// 邮件配置
	SMTPHost     string `json:"smtpHost"`
	SMTPPort     int    `json:"smtpPort"`
	SMTPUser     string `json:"smtpUser"`
	SMTPPassword string `json:"smtpPassword"`
	FromEmail    string `json:"fromEmail"`
}

// Person 人员信息
type Person struct {
	Name   string `json:"name"`
	Email  string `json:"email"`
	Avatar string `json:"avatar,omitempty"`
}

// Response 通用响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

var (
	config     Config
	configLock sync.RWMutex
	uploadDir  = "uploads"
	db         *sql.DB
	dbFile     = "checkin.db"
)

// loadConfig 加载配置文件
func loadConfig(filename string) error {
	configLock.Lock()
	defer configLock.Unlock()

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	return nil
}

// getConfig 获取配置（线程安全）
func getConfig() Config {
	configLock.RLock()
	defer configLock.RUnlock()
	return config
}

// initDB 初始化数据库
func initDB() error {
	var err error
	db, err = sql.Open("sqlite", dbFile)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}

	// 创建打卡记录表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS checkin_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		date TEXT NOT NULL,
		file_path TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(name, date)
	);
	`
	if _, err := db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("创建表失败: %w", err)
	}

	return nil
}

// saveCheckInRecord 保存打卡记录到数据库
func saveCheckInRecord(name string, date string, filePath string) error {
	// 使用 REPLACE 实现插入或更新
	query := `INSERT OR REPLACE INTO checkin_records (name, date, file_path, created_at) VALUES (?, ?, ?, ?)`
	_, err := db.Exec(query, name, date, filePath, time.Now())
	return err
}

// deleteCheckInRecords 删除指定日期的所有打卡记录
func deleteCheckInRecords(date string) error {
	_, err := db.Exec("DELETE FROM checkin_records WHERE date = ?", date)
	return err
}

// md5Hash 计算字符串的MD5哈希（16位小写）
func md5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	fullHash := hex.EncodeToString(hash[:])
	// 取16位小写（取中间16位）
	return strings.ToLower(fullHash[8:24])
}

// tokenAuthMiddleware Token验证中间件
func tokenAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			token = c.GetHeader("X-Token")
		}
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Response{
				Code:    401,
				Message: "缺少token参数",
			})
			return
		}

		cfg := getConfig()
		tokenHash := md5Hash(token)

		if tokenHash != cfg.TokenMd5 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Response{
				Code:    401,
				Message: "token验证失败",
			})
			return
		}

		c.Next()
	}
}

// getCheckInPersonList 获取打卡人员列表
func getCheckInPersonList(c *gin.Context) {
	cfg := getConfig()
	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "success",
		Data:    cfg.CheckInPersonList,
	})
}

// getUploadFilePath 获取上传文件路径
func getUploadFilePath(name string, date string, ext string) string {
	// 使用日期和名字作为文件名，支持当天覆盖
	safeFileName := strings.ReplaceAll(name, " ", "_")
	return filepath.Join(uploadDir, date, fmt.Sprintf("%s%s", safeFileName, ext))
}

// uploadCheckInProof 上传打卡凭证
func uploadCheckInProof(c *gin.Context) {
	name := c.PostForm("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "缺少name参数",
		})
		return
	}

	// 验证name是否在checkInPersonList中
	cfg := getConfig()
	nameValid := false
	for _, person := range cfg.CheckInPersonList {
		if person.Name == name {
			nameValid = true
			break
		}
	}

	if !nameValid {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "name不在打卡人员列表中",
		})
		return
	}

	// 获取上传的文件
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "请上传图片文件",
		})
		return
	}

	// 验证文件类型
	contentType := file.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "只支持上传图片文件",
		})
		return
	}

	// 获取文件扩展名
	ext := filepath.Ext(file.Filename)
	if ext == "" {
		ext = ".jpg"
	}

	// 获取今天的日期
	today := time.Now().Format("2006-01-02")

	// 创建日期目录
	dateDir := filepath.Join(uploadDir, today)
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "创建目录失败",
		})
		return
	}

	// 保存文件（当天覆盖）
	filePath := getUploadFilePath(name, today, ext)

	// 删除可能存在的旧文件（不同扩展名）
	deleteExistingFiles(name, today)

	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "保存文件失败",
		})
		return
	}

	// 保存打卡记录到数据库
	if err := saveCheckInRecord(name, today, filePath); err != nil {
		log.Printf("保存打卡记录到数据库失败: %v", err)
	}

	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "上传成功",
		Data: map[string]string{
			"name":     name,
			"date":     today,
			"filePath": filePath,
		},
	})
}

// deleteExistingFiles 删除用户当天已存在的文件
func deleteExistingFiles(name string, date string) {
	safeFileName := strings.ReplaceAll(name, " ", "_")
	dateDir := filepath.Join(uploadDir, date)
	pattern := filepath.Join(dateDir, safeFileName+".*")
	matches, _ := filepath.Glob(pattern)
	for _, match := range matches {
		os.Remove(match)
	}
}

// checkUserUploaded 检查用户是否已上传打卡凭证（从数据库查询）
func checkUserUploaded(name string, date string) bool {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM checkin_records WHERE name = ? AND date = ?", name, date).Scan(&count)
	if err != nil {
		log.Printf("查询打卡记录失败: %v", err)
		return false
	}
	return count > 0
}

// sendEmail 发送邮件
func sendEmail(to []string, subject string, body string) error {
	cfg := getConfig()

	if cfg.SMTPHost == "" || cfg.SMTPUser == "" || cfg.SMTPPassword == "" {
		log.Printf("邮件配置不完整，跳过发送邮件。收件人: %v, 主题: %s", to, subject)
		return nil
	}

	auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPHost)

	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"Content-Type: text/plain; charset=UTF-8\r\n"+
		"\r\n"+
		"%s", cfg.FromEmail, strings.Join(to, ","), subject, body)

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	return smtp.SendMail(addr, auth, cfg.FromEmail, to, []byte(msg))
}

// mentionTask 提醒任务：检查未上传用户并发送个人提醒邮件
func mentionTask() {
	cfg := getConfig()
	today := time.Now().Format("2006-01-02")

	log.Printf("执行提醒任务，日期: %s", today)

	for _, person := range cfg.CheckInPersonList {
		if !checkUserUploaded(person.Name, today) {
			if person.Email != "" {
				subject := "打卡提醒"
				body := fmt.Sprintf("亲爱的 %s：\n\n您今天还未完成打卡，请尽快上传打卡凭证。\n\n此邮件为系统自动发送，请勿回复。", person.Name)
				if err := sendEmail([]string{person.Email}, subject, body); err != nil {
					log.Printf("发送提醒邮件给 %s 失败: %v", person.Name, err)
				} else {
					log.Printf("已发送提醒邮件给 %s (%s)", person.Name, person.Email)
				}
			} else {
				log.Printf("用户 %s 未配置邮箱，跳过发送提醒", person.Name)
			}
		} else {
			log.Printf("用户 %s 已完成打卡", person.Name)
		}
	}
}

// checkInTask 打卡检查任务：检查未上传用户并群发邮件
func checkInTask() {
	cfg := getConfig()
	today := time.Now().Format("2006-01-02")

	log.Printf("执行打卡检查任务，日期: %s", today)

	var notUploaded []Person
	var allEmails []string

	for _, person := range cfg.CheckInPersonList {
		if person.Email != "" {
			allEmails = append(allEmails, person.Email)
		}
		if !checkUserUploaded(person.Name, today) {
			notUploaded = append(notUploaded, person)
		}
	}

	if len(notUploaded) > 0 && len(allEmails) > 0 {
		subject := "每日打卡未完成人员通知"
		var bodyBuilder strings.Builder
		bodyBuilder.WriteString("以下人员今日未完成打卡：\n\n")
		for _, person := range notUploaded {
			bodyBuilder.WriteString(fmt.Sprintf("姓名: %s, 邮箱: %s\n", person.Name, person.Email))
		}
		bodyBuilder.WriteString("\n此邮件为系统自动发送，请勿回复。")

		if err := sendEmail(allEmails, subject, bodyBuilder.String()); err != nil {
			log.Printf("发送群发邮件失败: %v", err)
		} else {
			log.Printf("已群发打卡检查结果邮件")
			deleteUploads()
		}
	} else if len(notUploaded) == 0 {
		log.Printf("所有人员已完成打卡")
		deleteUploads()
	} else {
		log.Printf("没有可用的邮箱地址，无法发送群发邮件")
	}
}

func deleteUploads() {
	today := time.Now().Format("2006-01-02")
	dateDir := filepath.Join(uploadDir, today) + ""
	if err := os.RemoveAll(dateDir); err != nil {
		log.Printf("删除上传目录失败: %v", err)
	} else {
		log.Printf("已删除上传目录: %s", dateDir)
		// 删除成功后重新创建目录
		if err := os.MkdirAll(dateDir, 0755); err != nil {
			log.Printf("重新创建上传目录失败: %v", err)
		}
	}
}

// startScheduler 启动定时任务调度器
func startScheduler() {
	go func() {
		for {
			now := time.Now()
			cfg := getConfig()

			// 解析mentionTime
			mentionTime, err := time.Parse("15:04", cfg.MentionTime)
			if err == nil {
				mentionDateTime := time.Date(now.Year(), now.Month(), now.Day(),
					mentionTime.Hour(), mentionTime.Minute(), 0, 0, now.Location())

				if now.Hour() == mentionDateTime.Hour() && now.Minute() == mentionDateTime.Minute() {
					mentionTask()
				}
			}

			// 解析checkInTime
			checkInTime, err := time.Parse("15:04", cfg.CheckInTime)
			if err == nil {
				checkInDateTime := time.Date(now.Year(), now.Month(), now.Day(),
					checkInTime.Hour(), checkInTime.Minute(), 0, 0, now.Location())

				if now.Hour() == checkInDateTime.Hour() && now.Minute() == checkInDateTime.Minute() {
					checkInTask()
				}
			}

			// 每分钟检查一次
			time.Sleep(time.Minute)
		}
	}()
}

// getCheckInStatus 获取打卡状态（额外提供的接口）
func getCheckInStatus(c *gin.Context) {
	date := c.Query("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	cfg := getConfig()
	var statusList []map[string]interface{}

	for _, person := range cfg.CheckInPersonList {
		uploaded := checkUserUploaded(person.Name, date)
		statusList = append(statusList, map[string]interface{}{
			"name":     person.Name,
			"avatar":   person.Avatar,
			"uploaded": uploaded,
		})
	}

	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "success",
		Data: map[string]interface{}{
			"date":   date,
			"status": statusList,
		},
	})
}

func main() {
	// 加载配置
	if err := loadConfig("config.json"); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	log.Println("配置加载成功")

	// 初始化数据库
	if err := initDB(); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer db.Close()
	log.Println("数据库初始化成功")

	// 创建上传目录
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("创建上传目录失败: %v", err)
	}

	// 启动定时任务
	startScheduler()
	log.Println("定时任务调度器已启动")

	// 创建Gin引擎
	r := gin.Default()

	// 根路径返回 index.html
	r.GET("/", func(c *gin.Context) {
		c.File("index.html")
	})

	// 根路径返回 avatar 图片
	r.GET("/avatar/:name", func(c *gin.Context) {
		name := c.Param("name")
		c.File("avatar/" + name)
		return
	})

	// API路由组（需要Token验证）
	api := r.Group("/api")
	api.Use(tokenAuthMiddleware())
	{

		// 获取打卡人员列表
		api.GET("/persons", getCheckInPersonList)

		// 上传打卡凭证
		api.POST("/upload", uploadCheckInProof)

		// 获取打卡状态
		api.GET("/status", getCheckInStatus)
	}

	// 启动服务器
	log.Println("服务器启动在 http://localhost:8989")
	if err := r.Run(":8989"); err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}
