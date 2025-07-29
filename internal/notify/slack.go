package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yourusername/sam-gov-monitor/internal/samgov"
)

// SlackNotifier implements Slack webhook notifications
type SlackNotifier struct {
	config  SlackConfig
	verbose bool
	client  *http.Client
}

// NewSlackNotifier creates a new Slack notifier
func NewSlackNotifier(config SlackConfig, verbose bool) *SlackNotifier {
	return &SlackNotifier{
		config:  config,
		verbose: verbose,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Send sends a Slack notification
func (sn *SlackNotifier) Send(ctx context.Context, notification Notification) error {
	if !sn.config.Enabled {
		return nil
	}

	if sn.verbose {
		log.Printf("Sending Slack notification: %s", notification.Subject)
	}

	// Build Slack message
	message, err := sn.buildSlackMessage(notification)
	if err != nil {
		return fmt.Errorf("building Slack message: %w", err)
	}

	// Send webhook request
	return sn.sendWebhook(ctx, message)
}

// GetType returns the notifier type
func (sn *SlackNotifier) GetType() string {
	return "slack"
}

// IsEnabled returns whether Slack notifications are enabled
func (sn *SlackNotifier) IsEnabled() bool {
	return sn.config.Enabled
}

// buildSlackMessage constructs a Slack message with blocks
func (sn *SlackNotifier) buildSlackMessage(notification Notification) (*SlackMessage, error) {
	message := &SlackMessage{
		Text: notification.Subject,
	}

	// Set channel if specified
	if sn.config.Channel != "" {
		message.Channel = sn.config.Channel
	}

	// Set username if specified
	if sn.config.Username != "" {
		message.Username = sn.config.Username
	} else {
		message.Username = "SAM.gov Monitor"
	}

	// Set icon
	if sn.config.IconEmoji != "" {
		message.IconEmoji = sn.config.IconEmoji
	} else {
		message.IconEmoji = ":rotating_light:"
	}

	// Build blocks
	blocks := sn.buildMessageBlocks(notification)
	message.Blocks = blocks

	return message, nil
}

// buildMessageBlocks creates Slack block kit blocks for the message
func (sn *SlackNotifier) buildMessageBlocks(notification Notification) []SlackBlock {
	blocks := make([]SlackBlock, 0)

	// Header block
	emoji := sn.getEmojiForPriority(notification.Priority)
	headerText := fmt.Sprintf("%s %s", emoji, notification.Subject)
	
	blocks = append(blocks, SlackBlock{
		Type: "header",
		Text: &SlackText{
			Type: "plain_text",
			Text: headerText,
		},
	})

	// Summary block
	summaryFields := make([]SlackField, 0)
	
	if notification.Summary.NewOpportunities > 0 {
		summaryFields = append(summaryFields, SlackField{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*New Opportunities:*\n%d", notification.Summary.NewOpportunities),
		})
	}
	
	if notification.Summary.UpdatedOpportunities > 0 {
		summaryFields = append(summaryFields, SlackField{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*Updated Opportunities:*\n%d", notification.Summary.UpdatedOpportunities),
		})
	}
	
	summaryFields = append(summaryFields, SlackField{
		Type: "mrkdwn",
		Text: fmt.Sprintf("*Priority:*\n%s", strings.Title(string(notification.Priority))),
	})
	
	if notification.Summary.UpcomingDeadlines > 0 {
		summaryFields = append(summaryFields, SlackField{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*Upcoming Deadlines:*\n%d", notification.Summary.UpcomingDeadlines),
		})
	}

	blocks = append(blocks, SlackBlock{
		Type:   "section",
		Fields: summaryFields,
	})

	// Opportunities blocks (limit to first 5 to avoid message size limits)
	maxOpportunities := 5
	for i, opp := range notification.Opportunities {
		if i >= maxOpportunities {
			remaining := len(notification.Opportunities) - maxOpportunities
			blocks = append(blocks, SlackBlock{
				Type: "context",
				Elements: []SlackElement{
					{
						Type: "mrkdwn",
						Text: fmt.Sprintf("_... and %d more opportunities_", remaining),
					},
				},
			})
			break
		}

		blocks = append(blocks, sn.buildOpportunityBlock(opp))
	}

	// Divider before footer
	blocks = append(blocks, SlackBlock{
		Type: "divider",
	})

	// Footer
	footerText := fmt.Sprintf("Generated on %s • Query: %s", 
		notification.Timestamp.Format("Jan 2, 2006 at 3:04 PM MST"), 
		notification.QueryName)
	
	blocks = append(blocks, SlackBlock{
		Type: "context",
		Elements: []SlackElement{
			{
				Type: "mrkdwn",
				Text: footerText,
			},
		},
	})

	return blocks
}

// buildOpportunityBlock creates a block for a single opportunity
func (sn *SlackNotifier) buildOpportunityBlock(opp samgov.Opportunity) SlackBlock {
	// Main text with title and basic info
	mainText := fmt.Sprintf("*<%s|%s>*\n", opp.UILink, opp.Title)
	mainText += fmt.Sprintf("Notice ID: `%s` • Type: %s • Posted: %s", 
		opp.NoticeID, opp.Type, opp.PostedDate)

	// Additional fields
	fields := make([]SlackField, 0)
	
	if opp.FullParentPath != "" {
		fields = append(fields, SlackField{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*Organization:*\n%s", opp.FullParentPath),
		})
	}
	
	if opp.ResponseDeadline != nil {
		deadlineEmoji := ":calendar:"
		if sn.isUrgentDeadline(*opp.ResponseDeadline) {
			deadlineEmoji = ":warning:"
		}
		fields = append(fields, SlackField{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*Deadline:* %s\n%s", deadlineEmoji, *opp.ResponseDeadline),
		})
	}
	
	if opp.TypeOfSetAside != "" {
		fields = append(fields, SlackField{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*Set-Aside:*\n%s", opp.TypeOfSetAside),
		})
	}
	
	if opp.NAICSCode != "" {
		fields = append(fields, SlackField{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*NAICS:*\n%s", opp.NAICSCode),
		})
	}

	return SlackBlock{
		Type: "section",
		Text: &SlackText{
			Type: "mrkdwn",
			Text: mainText,
		},
		Fields: fields,
		Accessory: &SlackAccessory{
			Type: "button",
			Text: &SlackText{
				Type: "plain_text",
				Text: "View on SAM.gov",
			},
			URL: opp.UILink,
		},
	}
}

// sendWebhook sends the message to Slack webhook
func (sn *SlackNotifier) sendWebhook(ctx context.Context, message *SlackMessage) error {
	// Marshal message to JSON
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshaling message: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", sn.config.WebhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := sn.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending webhook: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	if sn.verbose {
		log.Printf("Slack notification sent successfully")
	}

	return nil
}

// getEmojiForPriority returns emoji for priority level
func (sn *SlackNotifier) getEmojiForPriority(priority Priority) string {
	switch priority {
	case PriorityHigh:
		return ":rotating_light:"
	case PriorityMedium:
		return ":warning:"
	case PriorityLow:
		return ":information_source:"
	default:
		return ":bell:"
	}
}

// isUrgentDeadline checks if a deadline is within 7 days
func (sn *SlackNotifier) isUrgentDeadline(deadlineStr string) bool {
	deadline, err := time.Parse("2006-01-02", deadlineStr)
	if err != nil {
		return false
	}
	
	return time.Until(deadline) <= 7*24*time.Hour
}

// Slack message structures

// SlackMessage represents a Slack webhook message
type SlackMessage struct {
	Text      string       `json:"text"`
	Channel   string       `json:"channel,omitempty"`
	Username  string       `json:"username,omitempty"`
	IconEmoji string       `json:"icon_emoji,omitempty"`
	IconURL   string       `json:"icon_url,omitempty"`
	Blocks    []SlackBlock `json:"blocks,omitempty"`
}

// SlackBlock represents a Slack block kit block
type SlackBlock struct {
	Type      string          `json:"type"`
	Text      *SlackText      `json:"text,omitempty"`
	Fields    []SlackField    `json:"fields,omitempty"`
	Elements  []SlackElement  `json:"elements,omitempty"`
	Accessory *SlackAccessory `json:"accessory,omitempty"`
}

// SlackText represents Slack text object
type SlackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// SlackField represents a Slack field
type SlackField struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// SlackElement represents a Slack element
type SlackElement struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// SlackAccessory represents a Slack accessory element
type SlackAccessory struct {
	Type string     `json:"type"`
	Text *SlackText `json:"text,omitempty"`
	URL  string     `json:"url,omitempty"`
}

// BuildSlackTestMessage creates a test message for validation
func BuildSlackTestMessage() *SlackMessage {
	return &SlackMessage{
		Text:      "SAM.gov Monitor Test",
		Username:  "SAM.gov Monitor",
		IconEmoji: ":white_check_mark:",
		Blocks: []SlackBlock{
			{
				Type: "header",
				Text: &SlackText{
					Type: "plain_text",
					Text: ":white_check_mark: SAM.gov Monitor Test",
				},
			},
			{
				Type: "section",
				Text: &SlackText{
					Type: "mrkdwn",
					Text: "This is a test message from your SAM.gov Monitor. If you're seeing this, your Slack integration is working correctly!",
				},
			},
			{
				Type: "context",
				Elements: []SlackElement{
					{
						Type: "mrkdwn",
						Text: fmt.Sprintf("Test sent at %s", time.Now().Format("Jan 2, 2006 at 3:04 PM MST")),
					},
				},
			},
		},
	}
}