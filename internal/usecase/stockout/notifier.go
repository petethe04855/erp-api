package stockout

import (
	"context"
	"fmt"
	"html"
	"log"
	"strings"
	"time"

	domainAuth "chawy-erp-api/internal/domain/auth"
	domainStockOut "chawy-erp-api/internal/domain/stockout"
)

// UserLister lists ERP users (to find the owners).
type UserLister interface {
	FindAll(ctx context.Context) ([]*domainAuth.User, error)
}

// NoticeSender sends one HTML email.
type NoticeSender interface {
	SendNotification(recipientEmail, subject, htmlBody string) error
}

// EmailNotifier emails every active owner when a count was short. It runs in
// the background and only logs failures (SMTP may not be set up); the count
// screen shows the batch as needs_review either way.
type EmailNotifier struct {
	users  UserLister
	sender NoticeSender
	// async runs the send; tests replace it to run inline.
	async func(func())
}

// NewEmailNotifier builds the notifier.
func NewEmailNotifier(users UserLister, sender NoticeSender) *EmailNotifier {
	return &EmailNotifier{users: users, sender: sender, async: func(f func()) { go f() }}
}

// NotifyReview implements ReviewNotifier.
func (n *EmailNotifier) NotifyReview(b domainStockOut.Batch) {
	if n == nil || n.users == nil || n.sender == nil {
		return
	}
	n.async(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		users, err := n.users.FindAll(ctx)
		if err != nil {
			log.Printf("[WARN] stock-out review notice: list users: %v", err)
			return
		}
		subject, body := reviewMessage(b)
		for _, u := range users {
			if u == nil || !u.IsActive || !strings.EqualFold(u.Role, "owner") || strings.TrimSpace(u.Email) == "" {
				continue
			}
			if err := n.sender.SendNotification(u.Email, subject, body); err != nil {
				log.Printf("[WARN] stock-out review notice to owner %d: %v", u.ID, err)
			}
		}
	})
}

func reviewMessage(b domainStockOut.Batch) (string, string) {
	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		loc = time.FixedZone("ICT", 7*3600)
	}
	var rows strings.Builder
	for _, l := range b.Lines {
		if l.Short <= 0 {
			continue
		}
		fmt.Fprintf(&rows, "<tr><td>%s</td><td style=\"text-align:right\">%d</td><td style=\"text-align:right\">%d</td><td style=\"text-align:right;color:#D71920\"><b>%d</b></td></tr>",
			html.EscapeString(l.Name), l.Qty, l.Deducted, l.Short)
	}
	subject := fmt.Sprintf("Chawy ERP: นับของออก %s สต็อกในระบบไม่พอ รอตรวจ", b.BatchNo)
	body := fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"></head>
<body style="font-family:Helvetica,Arial,sans-serif;color:#111;padding:16px">
<p><b>%s</b> นับโดย %s เวลา %s</p>
<p>ของที่นับออกจริงมากกว่าที่ระบบมี ระบบตัดเท่าที่มีและไม่ปล่อยให้ติดลบ ให้นับสต็อกจริงแล้วปรับยอดในหน้า Inventory จากนั้นกด "ตรวจแล้ว" ในหน้านับของออก แท็บวันนี้</p>
<table cellpadding="6" style="border-collapse:collapse;border:1px solid #ddd">
<tr><th align="left">สินค้า</th><th>นับออก</th><th>ตัดได้</th><th>ขาด</th></tr>%s</table>
</body></html>`, html.EscapeString(b.BatchNo), html.EscapeString(b.UserName), b.CountedAt.In(loc).Format("02/01/2006 15:04"), rows.String())
	return subject, body
}
