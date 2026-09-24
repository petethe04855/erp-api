package mailer

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strings"
	"time"
)

type Mailer interface {
	SendUserCredentials(recipientEmail, recipientName, generatedPassword, role string) error
}

type smtpMailer struct {
	host     string
	port     string
	username string
	password string
	from     string
}

func NewMailer(host, port, username, password, emailSend string) Mailer {
	from := strings.TrimSpace(emailSend)
	if from == "" {
		from = strings.TrimSpace(username)
	}
	return &smtpMailer{
		host:     strings.TrimSpace(host),
		port:     strings.TrimSpace(port),
		username: strings.TrimSpace(username),
		password: strings.TrimSpace(password),
		from:     from,
	}
}

func (m *smtpMailer) SendUserCredentials(recipientEmail, recipientName, generatedPassword, role string) error {
	if m.host == "" || m.from == "" {
		log.Printf("[WARN] SMTP is not configured (SMTP_HOST or EMAIL_SEND is missing). Skipping email delivery to %s", recipientEmail)
		return nil
	}

	subject := "ยินดีต้อนรับสู่ระบบ Chawy ERP - ข้อมูลบัญชีผู้ใช้งานของคุณ"
	displayName := recipientName
	if strings.TrimSpace(displayName) == "" {
		displayName = recipientEmail
	}

	roleName := role
	switch role {
	case "owner":
		roleName = "เจ้าของกิจการ (Owner)"
	case "accountant":
		roleName = "ฝ่ายบัญชี (Accountant)"
	case "sales":
		roleName = "ฝ่ายขาย (Sales)"
	case "warehouse":
		roleName = "ฝ่ายคลังสินค้า (Warehouse)"
	case "live":
		roleName = "พนักงานไลฟ์ (Live Streamer)"
	}

	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
  body { font-family: 'Helvetica Neue', Helvetica, Arial, sans-serif; background-color: #f4f5f7; margin: 0; padding: 24px; color: #333333; }
  .card { max-width: 540px; margin: 0 auto; background: #ffffff; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 12px rgba(0,0,0,0.06); border: 1px solid #e5e7eb; }
  .header { background: #4f46e5; color: #ffffff; padding: 24px; text-align: center; }
  .header h1 { margin: 0; font-size: 20px; font-weight: 700; }
  .content { padding: 28px 24px; }
  .info-box { background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 8px; padding: 16px; margin: 20px 0; }
  .info-row { display: flex; justify-content: space-between; margin-bottom: 8px; font-size: 14px; }
  .info-label { color: #64748b; font-weight: 500; }
  .info-val { color: #0f172a; font-weight: 600; font-family: monospace; }
  .password-box { background: #eef2ff; border: 1px dashed #6366f1; border-radius: 8px; padding: 14px; text-align: center; margin: 16px 0; }
  .password-text { font-size: 20px; font-weight: bold; color: #4338ca; letter-spacing: 1.5px; font-family: monospace; }
  .footer { text-align: center; padding: 18px; font-size: 12px; color: #94a3b8; border-top: 1px solid #f1f5f9; background: #fafafa; }
</style>
</head>
<body>
  <div class="card">
    <div class="header">
      <h1>Chawy ERP</h1>
      <p style="margin: 4px 0 0 0; font-size: 13px; opacity: 0.9;">แจ้งข้อมูลบัญชีผู้ใช้งานใหม่</p>
    </div>
    <div class="content">
      <p>เรียนคุณ <strong>%s</strong>,</p>
      <p>ผู้ดูแลระบบได้สร้างบัญชีผู้ใช้งานสำหรับคุณในระบบ <strong>Chawy ERP</strong> เรียบร้อยแล้ว รายละเอียดการเข้าสู่ระบบมีดังนี้:</p>

      <div class="info-box">
        <div class="info-row"><span class="info-label">อีเมลผู้ใช้งาน:</span> <span class="info-val">%s</span></div>
        <div class="info-row"><span class="info-label">บทบาท/สิทธิ์:</span> <span class="info-val">%s</span></div>
      </div>

      <p style="margin-bottom: 6px; font-size: 13px; color: #475569;">รหัสผ่านชั่วคราวของคุณ:</p>
      <div class="password-box">
        <div class="password-text">%s</div>
      </div>

      <p style="font-size: 12px; color: #64748b; line-height: 1.5;">
        * เพื่อความปลอดภัย กรุณาเข้าสู่ระบบและเปลี่ยนรหัสผ่านทันทีหลังจากการเข้าสู่ระบบครั้งแรก<br>
        * หากคุณไม่ได้เป็นผู้ร้องขอการสร้างบัญชีนี้ กรุณาติดต่อผู้ดูแลระบบทันที
      </p>
    </div>
    <div class="footer">
      &copy; %d Chawy ERP. All rights reserved.
    </div>
  </div>
</body>
</html>`,
		displayName,
		recipientEmail,
		roleName,
		generatedPassword,
		time.Now().Year(),
	)

	headers := make(map[string]string)
	headers["From"] = m.from
	headers["To"] = recipientEmail
	headers["Subject"] = "=?UTF-8?B?" + fmt.Sprintf("%s", subject) + "?="
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"

	// RFC 2047 Subject encoding
	headerStr := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n",
		m.from, recipientEmail, subject)

	msg := []byte(headerStr + htmlBody)
	addr := fmt.Sprintf("%s:%s", m.host, m.port)

	// Send via SMTP
	var auth smtp.Auth
	if m.username != "" && m.password != "" {
		auth = smtp.PlainAuth("", m.username, m.password, m.host)
	}

	// Port 465 uses SSL/TLS directly, 587 uses STARTTLS
	if m.port == "465" {
		tlsconfig := &tls.Config{
			InsecureSkipVerify: false,
			ServerName:         m.host,
		}
		conn, err := tls.Dial("tcp", addr, tlsconfig)
		if err != nil {
			return fmt.Errorf("failed to dial SMTP TLS: %w", err)
		}
		defer conn.Close()

		c, err := smtp.NewClient(conn, m.host)
		if err != nil {
			return fmt.Errorf("failed to create SMTP client: %w", err)
		}
		defer c.Quit()

		if auth != nil {
			if ok, _ := c.Extension("AUTH"); ok {
				if err = c.Auth(auth); err != nil {
					return fmt.Errorf("SMTP auth failed: %w", err)
				}
			}
		}

		if err = c.Mail(m.from); err != nil {
			return fmt.Errorf("SMTP mail from failed: %w", err)
		}
		if err = c.Rcpt(recipientEmail); err != nil {
			return fmt.Errorf("SMTP rcpt to failed: %w", err)
		}
		w, err := c.Data()
		if err != nil {
			return fmt.Errorf("SMTP data write failed: %w", err)
		}
		_, err = w.Write(msg)
		if err != nil {
			return fmt.Errorf("SMTP write body failed: %w", err)
		}
		err = w.Close()
		if err != nil {
			return fmt.Errorf("SMTP close data failed: %w", err)
		}
		log.Printf("[INFO] Credentials email sent successfully to %s (via SSL/TLS port 465)", recipientEmail)
		return nil
	}

	// Standard STARTTLS / port 587 or 25
	// Using standard net.Dial with timeout
	conn, err := net.DialTimeout("tcp", addr, 15*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	c, err := smtp.NewClient(conn, m.host)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer c.Quit()

	if ok, _ := c.Extension("STARTTLS"); ok {
		config := &tls.Config{ServerName: m.host}
		if err = c.StartTLS(config); err != nil {
			return fmt.Errorf("SMTP StartTLS failed: %w", err)
		}
	}

	if auth != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err = c.Auth(auth); err != nil {
				return fmt.Errorf("SMTP auth failed: %w", err)
			}
		}
	}

	if err = c.Mail(m.from); err != nil {
		return fmt.Errorf("SMTP mail from failed: %w", err)
	}
	if err = c.Rcpt(recipientEmail); err != nil {
		return fmt.Errorf("SMTP rcpt to failed: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("SMTP data write failed: %w", err)
	}
	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("SMTP write body failed: %w", err)
	}
	err = w.Close()
	if err != nil {
		return fmt.Errorf("SMTP close data failed: %w", err)
	}

	log.Printf("[INFO] Credentials email sent successfully to %s", recipientEmail)
	return nil
}
