package samgov

import "time"

// Opportunity represents a SAM.gov opportunity
type Opportunity struct {
	NoticeID         string     `json:"noticeId"`
	Title            string     `json:"title"`
	SolicitationNum  string     `json:"solicitationNumber"`
	FullParentPath   string     `json:"fullParentPathName"`
	PostedDate       string     `json:"postedDate"`
	Type             string     `json:"type"`
	ResponseDeadline *string    `json:"responseDeadLine"`
	UILink           string     `json:"uiLink"`
	Active           string     `json:"active"`
	Description      string     `json:"description"`
	PointOfContact   []Contact  `json:"pointOfContact"`
	Award            *Award     `json:"award,omitempty"`
	PlaceOfPerformance *Place   `json:"placeOfPerformance,omitempty"`
	TypeOfSetAside   string     `json:"typeOfSetAside"`
	NAICSCode        string     `json:"naicsCode"`
}

// Contact represents a point of contact for an opportunity
type Contact struct {
	FullName     string `json:"fullname"`
	Title        string `json:"title"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	Fax          string `json:"fax"`
	Type         string `json:"type"`
}

// Award contains award information if available
type Award struct {
	Date   string  `json:"date"`
	Number string  `json:"number"`
	Amount float64 `json:"amount"`
}

// Place represents place of performance
type Place struct {
	StreetAddress string `json:"streetAddress"`
	City          string `json:"city"`
	State         string `json:"state"`
	ZipCode       string `json:"zip"`
	Country       string `json:"country"`
}

// SearchResponse represents the response from SAM.gov search API
type SearchResponse struct {
	TotalRecords      int           `json:"totalRecords"`
	Limit            int           `json:"limit"`
	Offset           int           `json:"offset"`
	OpportunitiesData []Opportunity `json:"opportunitiesData"`
}

// QueryResult represents the result of executing a search query
type QueryResult struct {
	QueryName     string        `json:"queryName"`
	Opportunities []Opportunity `json:"opportunities"`
	ExecutionTime time.Duration `json:"executionTime"`
	Error         error         `json:"error,omitempty"`
}

// DiffResult represents the difference between current and previous opportunities
type DiffResult struct {
	New      []Opportunity `json:"new"`
	Updated  []Opportunity `json:"updated"`
	Existing []Opportunity `json:"existing"`
}

// OpportunityState tracks the state of an opportunity over time
type OpportunityState struct {
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
	LastModified time.Time `json:"last_modified"`
	NoticeID     string    `json:"notice_id"`
	Title        string    `json:"title"`
	Deadline     *string   `json:"deadline,omitempty"`
	Hash         string    `json:"hash"`
}

// APIError represents an error from the SAM.gov API
type APIError struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Details    string `json:"details"`
}

func (e APIError) Error() string {
	return e.Message
}