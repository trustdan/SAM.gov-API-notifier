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
	StreetAddress string      `json:"streetAddress"`
	City          interface{} `json:"city"` // Can be string or object
	State         interface{} `json:"state"`
	ZipCode       interface{} `json:"zip"`
	Country       interface{} `json:"country"`
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

// GetCity returns the city as a string, handling both string and object types
func (p *Place) GetCity() string {
	if p == nil || p.City == nil {
		return ""
	}
	
	switch city := p.City.(type) {
	case string:
		return city
	case map[string]interface{}:
		// Handle object case - check for common fields
		if name, ok := city["name"].(string); ok {
			return name
		}
		if value, ok := city["value"].(string); ok {
			return value
		}
		// Try to find any string value in the map
		for _, v := range city {
			if str, ok := v.(string); ok && str != "" {
				return str
			}
		}
	}
	
	return ""
}

// GetState safely extracts state string from interface{}
func (p *Place) GetState() string {
	if p.State == nil {
		return ""
	}
	
	switch v := p.State.(type) {
	case string:
		return v
	case map[string]interface{}:
		// Handle object format - extract state name from nested structure
		if stateName, ok := v["state"].(string); ok {
			return stateName
		}
		// Try other possible field names
		if name, ok := v["name"].(string); ok {
			return name
		}
		// Try abbreviation field
		if abbr, ok := v["abbreviation"].(string); ok {
			return abbr
		}
	}
	return ""
}

// GetZipCode safely extracts zip code string from interface{}
func (p *Place) GetZipCode() string {
	if p.ZipCode == nil {
		return ""
	}
	
	switch v := p.ZipCode.(type) {
	case string:
		return v
	case map[string]interface{}:
		// Handle object format
		if zip, ok := v["zip"].(string); ok {
			return zip
		}
		if code, ok := v["code"].(string); ok {
			return code
		}
		if value, ok := v["value"].(string); ok {
			return value
		}
	}
	return ""
}

// GetCountry safely extracts country string from interface{}
func (p *Place) GetCountry() string {
	if p.Country == nil {
		return ""
	}
	
	switch v := p.Country.(type) {
	case string:
		return v
	case map[string]interface{}:
		// Handle object format - try different field names
		if countryName, ok := v["name"].(string); ok {
			return countryName
		}
		if code, ok := v["code"].(string); ok {
			return code
		}
		if country, ok := v["country"].(string); ok {
			return country
		}
	}
	return ""
}