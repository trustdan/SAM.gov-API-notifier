package notify

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"log"
	"net/smtp"
	"strings"
	"time"

	"github.com/yourusername/sam-gov-monitor/internal/samgov"
)

// EmailNotifier implements email notifications via SMTP
type EmailNotifier struct {
	config   EmailConfig
	verbose  bool
	templates *template.Template
}

// NewEmailNotifier creates a new email notifier
func NewEmailNotifier(config EmailConfig, verbose bool) *EmailNotifier {
	notifier := &EmailNotifier{
		config:  config,
		verbose: verbose,
	}
	
	notifier.loadTemplates()
	return notifier
}

// Send sends an email notification
func (en *EmailNotifier) Send(ctx context.Context, notification Notification) error {
	if !en.config.Enabled {
		return nil
	}

	if en.verbose {
		log.Printf("Sending email notification: %s", notification.Subject)
	}

	// Build email content
	emailBody, err := en.buildEmailBody(notification)
	if err != nil {
		return fmt.Errorf("building email body: %w", err)
	}

	// Prepare recipients
	recipients := notification.Recipients
	if len(recipients) == 0 {
		recipients = en.config.ToAddresses
	}
	
	if len(recipients) == 0 {
		return fmt.Errorf("no email recipients specified")
	}

	// Build email message
	message, err := en.buildMessage(notification, emailBody, recipients)
	if err != nil {
		return fmt.Errorf("building email message: %w", err)
	}

	// Send email
	return en.sendSMTP(recipients, message)
}

// GetType returns the notifier type
func (en *EmailNotifier) GetType() string {
	return "email"
}

// IsEnabled returns whether email notifications are enabled
func (en *EmailNotifier) IsEnabled() bool {
	return en.config.Enabled
}

// buildEmailBody generates the email body using templates
func (en *EmailNotifier) buildEmailBody(notification Notification) (string, error) {
	// Prepare template data
	data := EmailTemplateData{
		QueryName:     notification.QueryName,
		Subject:       notification.Subject,
		Opportunities: notification.Opportunities,
		Summary:       notification.Summary,
		Priority:      string(notification.Priority),
		Timestamp:     notification.Timestamp,
		PriorityClass: en.getPriorityClass(notification.Priority),
	}

	// Choose template based on priority and content
	templateName := "opportunity"
	if notification.Summary.UpdatedOpportunities > 0 {
		templateName = "opportunity-updated"
	}

	// Execute template
	var buf bytes.Buffer
	if err := en.templates.ExecuteTemplate(&buf, templateName, data); err != nil {
		return "", fmt.Errorf("executing template %s: %w", templateName, err)
	}

	return buf.String(), nil
}

// buildMessage constructs the complete email message
func (en *EmailNotifier) buildMessage(notification Notification, body string, recipients []string) ([]byte, error) {
	var message bytes.Buffer

	// Headers
	message.WriteString(fmt.Sprintf("From: %s\r\n", en.config.FromAddress))
	message.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(recipients, ", ")))
	message.WriteString(fmt.Sprintf("Subject: %s\r\n", notification.Subject))
	message.WriteString("MIME-Version: 1.0\r\n")
	message.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	message.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	
	// Add priority headers for high-priority notifications
	if notification.Priority == PriorityHigh {
		message.WriteString("X-Priority: 1\r\n")
		message.WriteString("X-MSMail-Priority: High\r\n")
		message.WriteString("Importance: High\r\n")
	}

	message.WriteString("\r\n")

	// Body
	message.WriteString(body)

	return message.Bytes(), nil
}

// sendSMTP sends the email via SMTP
func (en *EmailNotifier) sendSMTP(recipients []string, message []byte) error {
	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", en.config.SMTPHost, en.config.SMTPPort)
	
	var client *smtp.Client
	var err error

	if en.config.UseTLS {
		// TLS connection
		tlsConfig := &tls.Config{
			ServerName: en.config.SMTPHost,
		}
		
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("TLS dial: %w", err)
		}
		defer conn.Close()

		client, err = smtp.NewClient(conn, en.config.SMTPHost)
		if err != nil {
			return fmt.Errorf("SMTP client: %w", err)
		}
	} else {
		// Plain connection with STARTTLS
		client, err = smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("SMTP dial: %w", err)
		}
		
		// Use STARTTLS if available
		if ok, _ := client.Extension("STARTTLS"); ok {
			config := &tls.Config{ServerName: en.config.SMTPHost}
			if err = client.StartTLS(config); err != nil {
				client.Close()
				return fmt.Errorf("STARTTLS: %w", err)
			}
		}
	}
	defer client.Close()

	// Authenticate
	if en.config.Username != "" && en.config.Password != "" {
		auth := smtp.PlainAuth("", en.config.Username, en.config.Password, en.config.SMTPHost)
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth: %w", err)
		}
	}

	// Send email
	if err = client.Mail(en.config.FromAddress); err != nil {
		return fmt.Errorf("MAIL command: %w", err)
	}

	for _, recipient := range recipients {
		if err = client.Rcpt(recipient); err != nil {
			return fmt.Errorf("RCPT command for %s: %w", recipient, err)
		}
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA command: %w", err)
	}

	_, err = writer.Write(message)
	if err != nil {
		writer.Close()
		return fmt.Errorf("writing message: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return fmt.Errorf("closing writer: %w", err)
	}

	if en.verbose {
		log.Printf("Email sent successfully to %d recipients", len(recipients))
	}

	return nil
}

// getPriorityClass returns CSS class for priority styling
func (en *EmailNotifier) getPriorityClass(priority Priority) string {
	switch priority {
	case PriorityHigh:
		return "high-priority"
	case PriorityMedium:
		return "medium-priority"
	case PriorityLow:
		return "low-priority"
	default:
		return ""
	}
}

// loadTemplates loads email templates
func (en *EmailNotifier) loadTemplates() {
	funcMap := template.FuncMap{
		"title": strings.Title,
	}
	
	en.templates = template.Must(template.New("email").Funcs(funcMap).Parse(opportunityTemplate))
	template.Must(en.templates.New("opportunity-updated").Funcs(funcMap).Parse(opportunityUpdatedTemplate))
}

// EmailTemplateData holds data for email templates
type EmailTemplateData struct {
	QueryName     string              `json:"query_name"`
	Subject       string              `json:"subject"`
	Opportunities []samgov.Opportunity `json:"opportunities"`
	Summary       NotificationSummary `json:"summary"`
	Priority      string              `json:"priority"`
	PriorityClass string              `json:"priority_class"`
	Timestamp     time.Time           `json:"timestamp"`
}

// Email templates
const opportunityTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>{{.Subject}}</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
        }
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 20px;
            border-radius: 8px 8px 0 0;
            text-align: center;
        }
        .header h1 {
            margin: 0;
            font-size: 24px;
        }
        .summary {
            background: #f8f9fa;
            padding: 15px;
            border-left: 4px solid #667eea;
            margin: 20px 0;
        }
        .opportunity {
            border: 1px solid #ddd;
            margin: 15px 0;
            border-radius: 8px;
            overflow: hidden;
        }
        .opportunity-header {
            background: #f8f9fa;
            padding: 15px;
            border-bottom: 1px solid #ddd;
        }
        .opportunity-content {
            padding: 15px;
        }
        .high-priority {
            border-left: 5px solid #dc3545;
        }
        .medium-priority {
            border-left: 5px solid #ffc107;
        }
        .low-priority {
            border-left: 5px solid #28a745;
        }
        .deadline {
            color: #dc3545;
            font-weight: bold;
            background: #fff5f5;
            padding: 5px 10px;
            border-radius: 4px;
            display: inline-block;
            margin: 5px 0;
        }
        .notice-id {
            font-family: monospace;
            background: #e9ecef;
            padding: 3px 6px;
            border-radius: 3px;
            font-size: 0.9em;
        }
        .btn {
            display: inline-block;
            padding: 8px 16px;
            background: #667eea;
            color: white;
            text-decoration: none;
            border-radius: 4px;
            margin: 5px 5px 5px 0;
        }
        .btn:hover {
            background: #5a6fd8;
        }
        .metadata {
            font-size: 0.9em;
            color: #666;
            margin: 5px 0;
        }
        .footer {
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #ddd;
            font-size: 0.9em;
            color: #666;
            text-align: center;
        }
        .stats {
            display: flex;
            justify-content: space-around;
            margin: 15px 0;
        }
        .stat-item {
            text-align: center;
        }
        .stat-number {
            font-size: 1.5em;
            font-weight: bold;
            color: #667eea;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>🚨 SAM.gov Opportunities Found</h1>
        <p><strong>{{.QueryName}}</strong> - {{.Summary.NewOpportunities}} New Opportunities</p>
    </div>

    <div class="summary">
        <div class="stats">
            <div class="stat-item">
                <div class="stat-number">{{.Summary.NewOpportunities}}</div>
                <div>New Opportunities</div>
            </div>
            {{if .Summary.UpcomingDeadlines}}
            <div class="stat-item">
                <div class="stat-number">{{.Summary.UpcomingDeadlines}}</div>
                <div>Upcoming Deadlines</div>
            </div>
            {{end}}
            <div class="stat-item">
                <div class="stat-number">{{.Priority | title}}</div>
                <div>Priority</div>
            </div>
        </div>
    </div>

    {{range .Opportunities}}
    <div class="opportunity {{$.PriorityClass}}">
        <div class="opportunity-header">
            <h3 style="margin: 0 0 10px 0;">{{.Title}}</h3>
            <div class="metadata">
                <span class="notice-id">{{.NoticeID}}</span>
                <span style="margin-left: 15px;"><strong>Type:</strong> {{.Type}}</span>
                <span style="margin-left: 15px;"><strong>Posted:</strong> {{.PostedDate}}</span>
            </div>
        </div>
        
        <div class="opportunity-content">
            {{if .ResponseDeadline}}
            <div class="deadline">
                ⏰ <strong>Response Deadline:</strong> {{.ResponseDeadline}}
            </div>
            {{end}}
            
            <div class="metadata" style="margin: 10px 0;">
                {{if .FullParentPath}}<div><strong>Organization:</strong> {{.FullParentPath}}</div>{{end}}
                {{if .TypeOfSetAside}}<div><strong>Set-Aside:</strong> {{.TypeOfSetAside}}</div>{{end}}
                {{if .NAICSCode}}<div><strong>NAICS Code:</strong> {{.NAICSCode}}</div>{{end}}
            </div>

            {{if .Description}}
            <div style="margin: 15px 0;">
                <strong>Description:</strong><br>
                <div style="background: #f8f9fa; padding: 10px; border-radius: 4px; margin-top: 5px;">
                    {{if gt (len .Description) 300}}
                        {{slice .Description 0 300}}...
                    {{else}}
                        {{.Description}}
                    {{end}}
                </div>
            </div>
            {{end}}

            <div style="margin-top: 15px;">
                <a href="{{.UILink}}" class="btn">View on SAM.gov</a>
            </div>
        </div>
    </div>
    {{end}}

    <div class="footer">
        <p>Generated on {{.Timestamp.Format "January 2, 2006 at 3:04 PM MST"}}</p>
        <p>🤖 Automated by <strong>SAM.gov Monitor</strong></p>
        <p style="font-size: 0.8em; margin-top: 10px;">
            This is an automated notification. Do not reply to this email.
        </p>
    </div>
</body>
</html>
`

const opportunityUpdatedTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>{{.Subject}}</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
        }
        .header {
            background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);
            color: white;
            padding: 20px;
            border-radius: 8px 8px 0 0;
            text-align: center;
        }
        .header h1 {
            margin: 0;
            font-size: 24px;
        }
        .opportunity {
            border: 1px solid #ddd;
            margin: 15px 0;
            border-radius: 8px;
            border-left: 5px solid #f5576c;
        }
        .opportunity-header {
            background: #fff5f5;
            padding: 15px;
            border-bottom: 1px solid #ddd;
        }
        .opportunity-content {
            padding: 15px;
        }
        .update-badge {
            background: #f5576c;
            color: white;
            padding: 3px 8px;
            border-radius: 12px;
            font-size: 0.8em;
            font-weight: bold;
        }
        .btn {
            display: inline-block;
            padding: 8px 16px;
            background: #f5576c;
            color: white;
            text-decoration: none;
            border-radius: 4px;
            margin: 5px 5px 5px 0;
        }
        .notice-id {
            font-family: monospace;
            background: #e9ecef;
            padding: 3px 6px;
            border-radius: 3px;
            font-size: 0.9em;
        }
        .metadata {
            font-size: 0.9em;
            color: #666;
            margin: 5px 0;
        }
        .footer {
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #ddd;
            font-size: 0.9em;
            color: #666;
            text-align: center;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>🔄 SAM.gov Opportunities Updated</h1>
        <p><strong>{{.QueryName}}</strong> - {{.Summary.UpdatedOpportunities}} Updated Opportunities</p>
    </div>

    {{range .Opportunities}}
    <div class="opportunity">
        <div class="opportunity-header">
            <h3 style="margin: 0 0 10px 0;">
                {{.Title}}
                <span class="update-badge">UPDATED</span>
            </h3>
            <div class="metadata">
                <span class="notice-id">{{.NoticeID}}</span>
                <span style="margin-left: 15px;"><strong>Posted:</strong> {{.PostedDate}}</span>
            </div>
        </div>
        
        <div class="opportunity-content">
            {{if .ResponseDeadline}}
            <div style="color: #f5576c; font-weight: bold; margin: 10px 0;">
                ⏰ <strong>Response Deadline:</strong> {{.ResponseDeadline}}
            </div>
            {{end}}
            
            <div style="margin-top: 15px;">
                <a href="{{.UILink}}" class="btn">View Changes on SAM.gov</a>
            </div>
        </div>
    </div>
    {{end}}

    <div class="footer">
        <p>Updated on {{.Timestamp.Format "January 2, 2006 at 3:04 PM MST"}}</p>
        <p>🤖 Automated by <strong>SAM.gov Monitor</strong></p>
    </div>
</body>
</html>
`