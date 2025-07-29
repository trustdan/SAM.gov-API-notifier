package notify

import (
	"fmt"
	"strings"
	"time"

	"github.com/yourusername/sam-gov-monitor/internal/samgov"
)

// CalendarGenerator creates iCalendar (.ics) files for opportunity deadlines
type CalendarGenerator struct {
	verbose bool
}

// NewCalendarGenerator creates a new calendar generator
func NewCalendarGenerator(verbose bool) *CalendarGenerator {
	return &CalendarGenerator{
		verbose: verbose,
	}
}

// GenerateICS creates an iCalendar file content for opportunities with deadlines
func (cg *CalendarGenerator) GenerateICS(opportunities []samgov.Opportunity, queryName string) string {
	var ics strings.Builder
	
	// iCalendar header
	ics.WriteString("BEGIN:VCALENDAR\r\n")
	ics.WriteString("VERSION:2.0\r\n")
	ics.WriteString("PRODID:-//SAM.gov Monitor//Opportunity Deadlines//EN\r\n")
	ics.WriteString("CALSCALE:GREGORIAN\r\n")
	ics.WriteString("METHOD:PUBLISH\r\n")
	ics.WriteString(fmt.Sprintf("X-WR-CALNAME:SAM.gov Opportunities - %s\r\n", queryName))
	ics.WriteString("X-WR-CALDESC:Deadlines for SAM.gov opportunities\r\n")
	ics.WriteString("X-WR-TIMEZONE:America/New_York\r\n")

	// Add timezone definition for EST/EDT
	cg.addTimezoneDefinition(&ics)

	eventCount := 0
	now := time.Now()
	
	for _, opp := range opportunities {
		if opp.ResponseDeadline == nil {
			continue
		}

		// Parse deadline
		deadline, err := time.Parse("2006-01-02", *opp.ResponseDeadline)
		if err != nil {
			continue
		}

		// Skip past deadlines
		if deadline.Before(now) {
			continue
		}

		// Create events: reminder and deadline
		cg.addReminderEvent(&ics, opp, deadline, queryName, 5)  // 5 days before
		cg.addReminderEvent(&ics, opp, deadline, queryName, 1)  // 1 day before
		cg.addDeadlineEvent(&ics, opp, deadline, queryName)     // Deadline day
		
		eventCount++
	}

	// iCalendar footer
	ics.WriteString("END:VCALENDAR\r\n")

	if cg.verbose && eventCount > 0 {
		fmt.Printf("Generated calendar with %d opportunities\n", eventCount)
	}

	return ics.String()
}

// addTimezoneDefinition adds Eastern Time zone definition
func (cg *CalendarGenerator) addTimezoneDefinition(ics *strings.Builder) {
	ics.WriteString("BEGIN:VTIMEZONE\r\n")
	ics.WriteString("TZID:America/New_York\r\n")
	ics.WriteString("BEGIN:DAYLIGHT\r\n")
	ics.WriteString("TZOFFSETFROM:-0500\r\n")
	ics.WriteString("TZOFFSETTO:-0400\r\n")
	ics.WriteString("TZNAME:EDT\r\n")
	ics.WriteString("DTSTART:20230312T070000\r\n")
	ics.WriteString("RRULE:FREQ=YEARLY;BYMONTH=3;BYDAY=2SU\r\n")
	ics.WriteString("END:DAYLIGHT\r\n")
	ics.WriteString("BEGIN:STANDARD\r\n")
	ics.WriteString("TZOFFSETFROM:-0400\r\n")
	ics.WriteString("TZOFFSETTO:-0500\r\n")
	ics.WriteString("TZNAME:EST\r\n")
	ics.WriteString("DTSTART:20231105T060000\r\n")
	ics.WriteString("RRULE:FREQ=YEARLY;BYMONTH=11;BYDAY=1SU\r\n")
	ics.WriteString("END:STANDARD\r\n")
	ics.WriteString("END:VTIMEZONE\r\n")
}

// addReminderEvent adds a reminder event before the deadline
func (cg *CalendarGenerator) addReminderEvent(ics *strings.Builder, opp samgov.Opportunity, deadline time.Time, queryName string, daysBefore int) {
	reminderDate := deadline.AddDate(0, 0, -daysBefore)
	
	// Skip if reminder is in the past
	if reminderDate.Before(time.Now()) {
		return
	}

	uid := fmt.Sprintf("reminder-%d-%s@samgov-monitor", daysBefore, opp.NoticeID)
	
	ics.WriteString("BEGIN:VEVENT\r\n")
	ics.WriteString(fmt.Sprintf("UID:%s\r\n", uid))
	ics.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", time.Now().UTC().Format("20060102T150405Z")))
	ics.WriteString(fmt.Sprintf("DTSTART;VALUE=DATE:%s\r\n", reminderDate.Format("20060102")))
	ics.WriteString(fmt.Sprintf("SUMMARY:%d Day Reminder: %s\r\n", daysBefore, cg.escapeText(opp.Title)))
	
	description := fmt.Sprintf("Reminder: SAM.gov opportunity deadline in %d days\\n\\n", daysBefore)
	description += fmt.Sprintf("Notice ID: %s\\n", opp.NoticeID)
	description += fmt.Sprintf("Organization: %s\\n", cg.escapeText(opp.FullParentPath))
	description += fmt.Sprintf("Deadline: %s\\n", deadline.Format("January 2, 2006"))
	description += fmt.Sprintf("View: %s", opp.UILink)
	
	ics.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", description))
	ics.WriteString(fmt.Sprintf("URL:%s\r\n", opp.UILink))
	ics.WriteString("CATEGORIES:SAM.gov,Reminder\r\n")
	ics.WriteString("PRIORITY:5\r\n")
	
	// Add alarm for high-priority reminders
	if daysBefore == 1 {
		ics.WriteString("BEGIN:VALARM\r\n")
		ics.WriteString("TRIGGER:-PT2H\r\n") // 2 hours before
		ics.WriteString("ACTION:DISPLAY\r\n")
		ics.WriteString(fmt.Sprintf("DESCRIPTION:SAM.gov deadline tomorrow: %s\r\n", cg.escapeText(opp.Title)))
		ics.WriteString("END:VALARM\r\n")
	}
	
	ics.WriteString("END:VEVENT\r\n")
}

// addDeadlineEvent adds the actual deadline event
func (cg *CalendarGenerator) addDeadlineEvent(ics *strings.Builder, opp samgov.Opportunity, deadline time.Time, queryName string) {
	uid := fmt.Sprintf("deadline-%s@samgov-monitor", opp.NoticeID)
	
	ics.WriteString("BEGIN:VEVENT\r\n")
	ics.WriteString(fmt.Sprintf("UID:%s\r\n", uid))
	ics.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", time.Now().UTC().Format("20060102T150405Z")))
	ics.WriteString(fmt.Sprintf("DTSTART;VALUE=DATE:%s\r\n", deadline.Format("20060102")))
	ics.WriteString(fmt.Sprintf("SUMMARY:🚨 DEADLINE: %s\r\n", cg.escapeText(opp.Title)))
	
	description := fmt.Sprintf("SAM.gov Opportunity Response Deadline\\n\\n")
	description += fmt.Sprintf("Notice ID: %s\\n", opp.NoticeID)
	description += fmt.Sprintf("Organization: %s\\n", cg.escapeText(opp.FullParentPath))
	description += fmt.Sprintf("Posted: %s\\n", opp.PostedDate)
	if opp.TypeOfSetAside != "" {
		description += fmt.Sprintf("Set-Aside: %s\\n", opp.TypeOfSetAside)
	}
	if opp.NAICSCode != "" {
		description += fmt.Sprintf("NAICS Code: %s\\n", opp.NAICSCode)
	}
	description += fmt.Sprintf("\\nView on SAM.gov: %s", opp.UILink)
	
	ics.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", description))
	ics.WriteString(fmt.Sprintf("URL:%s\r\n", opp.UILink))
	ics.WriteString("CATEGORIES:SAM.gov,Deadline,Urgent\r\n")
	ics.WriteString("PRIORITY:1\r\n") // High priority
	ics.WriteString("STATUS:CONFIRMED\r\n")
	
	// Add multiple alarms for deadline day
	cg.addDeadlineAlarms(ics)
	
	ics.WriteString("END:VEVENT\r\n")
}

// addDeadlineAlarms adds multiple alarms for deadline events
func (cg *CalendarGenerator) addDeadlineAlarms(ics *strings.Builder) {
	// Morning alarm
	ics.WriteString("BEGIN:VALARM\r\n")
	ics.WriteString("TRIGGER;VALUE=DATE-TIME:20240101T090000Z\r\n") // Will be relative to event
	ics.WriteString("ACTION:DISPLAY\r\n")
	ics.WriteString("DESCRIPTION:SAM.gov response deadline TODAY!\r\n")
	ics.WriteString("END:VALARM\r\n")
	
	// Afternoon reminder
	ics.WriteString("BEGIN:VALARM\r\n")
	ics.WriteString("TRIGGER:-PT6H\r\n") // 6 hours before end of day
	ics.WriteString("ACTION:DISPLAY\r\n")
	ics.WriteString("DESCRIPTION:Final hours to submit SAM.gov response!\r\n")
	ics.WriteString("END:VALARM\r\n")
}

// GenerateDeadlineOnlyICS creates a simplified ICS with only deadline events
func (cg *CalendarGenerator) GenerateDeadlineOnlyICS(opportunities []samgov.Opportunity, queryName string) string {
	var ics strings.Builder
	
	// iCalendar header
	ics.WriteString("BEGIN:VCALENDAR\r\n")
	ics.WriteString("VERSION:2.0\r\n")
	ics.WriteString("PRODID:-//SAM.gov Monitor//Deadlines Only//EN\r\n")
	ics.WriteString("CALSCALE:GREGORIAN\r\n")
	ics.WriteString(fmt.Sprintf("X-WR-CALNAME:%s Deadlines\r\n", queryName))

	now := time.Now()
	
	for _, opp := range opportunities {
		if opp.ResponseDeadline == nil {
			continue
		}

		deadline, err := time.Parse("2006-01-02", *opp.ResponseDeadline)
		if err != nil || deadline.Before(now) {
			continue
		}

		cg.addSimpleDeadlineEvent(&ics, opp, deadline)
	}

	ics.WriteString("END:VCALENDAR\r\n")
	return ics.String()
}

// addSimpleDeadlineEvent adds a simple deadline event without alarms
func (cg *CalendarGenerator) addSimpleDeadlineEvent(ics *strings.Builder, opp samgov.Opportunity, deadline time.Time) {
	uid := fmt.Sprintf("simple-%s@samgov-monitor", opp.NoticeID)
	
	ics.WriteString("BEGIN:VEVENT\r\n")
	ics.WriteString(fmt.Sprintf("UID:%s\r\n", uid))
	ics.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", time.Now().UTC().Format("20060102T150405Z")))
	ics.WriteString(fmt.Sprintf("DTSTART;VALUE=DATE:%s\r\n", deadline.Format("20060102")))
	ics.WriteString(fmt.Sprintf("SUMMARY:%s (%s)\r\n", cg.escapeText(opp.Title), opp.NoticeID))
	ics.WriteString(fmt.Sprintf("URL:%s\r\n", opp.UILink))
	ics.WriteString("CATEGORIES:SAM.gov\r\n")
	ics.WriteString("END:VEVENT\r\n")
}

// CreateCalendarAttachment creates a calendar attachment for notifications
func (cg *CalendarGenerator) CreateCalendarAttachment(opportunities []samgov.Opportunity, queryName string) Attachment {
	icsContent := cg.GenerateICS(opportunities, queryName)
	
	fileName := fmt.Sprintf("%s-deadlines.ics", strings.ReplaceAll(queryName, " ", "-"))
	
	return Attachment{
		Name:        fileName,
		Content:     []byte(icsContent),
		ContentType: "text/calendar",
	}
}

// GetUpcomingDeadlines returns opportunities with deadlines in the next N days
func (cg *CalendarGenerator) GetUpcomingDeadlines(opportunities []samgov.Opportunity, days int) []samgov.Opportunity {
	upcoming := make([]samgov.Opportunity, 0)
	
	now := time.Now()
	cutoff := now.AddDate(0, 0, days)
	
	for _, opp := range opportunities {
		if opp.ResponseDeadline == nil {
			continue
		}
		
		deadline, err := time.Parse("2006-01-02", *opp.ResponseDeadline)
		if err != nil {
			continue
		}
		
		if deadline.After(now) && deadline.Before(cutoff) {
			upcoming = append(upcoming, opp)
		}
	}
	
	return upcoming
}

// escapeText escapes special characters for iCalendar format
func (cg *CalendarGenerator) escapeText(text string) string {
	// iCalendar text escaping rules
	text = strings.ReplaceAll(text, "\\", "\\\\")
	text = strings.ReplaceAll(text, ",", "\\,")
	text = strings.ReplaceAll(text, ";", "\\;")
	text = strings.ReplaceAll(text, "\n", "\\n")
	text = strings.ReplaceAll(text, "\r", "")
	
	// Limit length to prevent calendar client issues
	if len(text) > 75 {
		text = text[:72] + "..."
	}
	
	return text
}

// ValidateDeadlineFormat checks if deadline string is in correct format
func (cg *CalendarGenerator) ValidateDeadlineFormat(deadline string) error {
	_, err := time.Parse("2006-01-02", deadline)
	if err != nil {
		return fmt.Errorf("invalid deadline format '%s', expected YYYY-MM-DD", deadline)
	}
	return nil
}

// DeadlineStats provides statistics about deadlines
type DeadlineStats struct {
	Total           int `json:"total"`
	WithinWeek      int `json:"within_week"`
	WithinMonth     int `json:"within_month"`
	Past            int `json:"past"`
	Average         int `json:"average_days_until"`
}

// GetDeadlineStats analyzes deadline distribution
func (cg *CalendarGenerator) GetDeadlineStats(opportunities []samgov.Opportunity) DeadlineStats {
	stats := DeadlineStats{}
	
	now := time.Now()
	totalDays := 0
	validDeadlines := 0
	
	for _, opp := range opportunities {
		if opp.ResponseDeadline == nil {
			continue
		}
		
		deadline, err := time.Parse("2006-01-02", *opp.ResponseDeadline)
		if err != nil {
			continue
		}
		
		stats.Total++
		daysUntil := int(deadline.Sub(now).Hours() / 24)
		
		if daysUntil < 0 {
			stats.Past++
		} else {
			validDeadlines++
			totalDays += daysUntil
			
			if daysUntil <= 7 {
				stats.WithinWeek++
			}
			if daysUntil <= 30 {
				stats.WithinMonth++
			}
		}
	}
	
	if validDeadlines > 0 {
		stats.Average = totalDays / validDeadlines
	}
	
	return stats
}