package models

import (
	"fmt"
	"time"
)

type Organization struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	OrgType   string    `json:"orgType"`
	SPID      *string   `json:"spid,omitempty"`
	RespOrgID *string   `json:"respOrgId,omitempty"`
	APIKey    *string   `json:"apiKey,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	FirstName    string    `json:"firstName"`
	LastName     string    `json:"lastName"`
	Role         string    `json:"role"`
	OrgID        *int64    `json:"orgId,omitempty"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type DNONumber struct {
	ID              int64     `json:"id"`
	PhoneNumber     string    `json:"phoneNumber"`
	Dataset         string    `json:"dataset"`
	NumberType      string    `json:"numberType"`
	Channel         string    `json:"channel"`
	Status          string    `json:"status"`
	Reason          *string   `json:"reason,omitempty"`
	AddedByOrgID    *int64    `json:"addedByOrgId,omitempty"`
	AddedByUser     *int64    `json:"addedByUserId,omitempty"`
	InvestigationID *string   `json:"investigationId,omitempty"`
	ThreatCategory  *string   `json:"threatCategory,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type QueryLog struct {
	ID          int64     `json:"id"`
	OrgID       int64     `json:"orgId"`
	PhoneNumber string    `json:"phoneNumber"`
	Result      string    `json:"result"`
	Channel     string    `json:"channel"`
	QueriedAt   time.Time `json:"queriedAt"`
}

type AuditLog struct {
	ID         int64     `json:"id"`
	UserID     *int64    `json:"userId,omitempty"`
	OrgID      *int64    `json:"orgId,omitempty"`
	Action     string    `json:"action"`
	EntityType string    `json:"entityType"`
	EntityID   *int64    `json:"entityId,omitempty"`
	Details    *string   `json:"details,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

type BulkJob struct {
	ID               int64      `json:"id"`
	OrgID            int64      `json:"orgId"`
	UserID           int64      `json:"userId"`
	JobType          string     `json:"jobType"`
	Status           string     `json:"status"`
	TotalRecords     int        `json:"totalRecords"`
	ProcessedRecords int        `json:"processedRecords"`
	SuccessCount     int        `json:"successCount"`
	ErrorCount       int        `json:"errorCount"`
	FileName         *string    `json:"fileName,omitempty"`
	ResultSummary    *string    `json:"resultSummary,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	CompletedAt      *time.Time `json:"completedAt,omitempty"`
}

// Request/Response types

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type DNOQueryRequest struct {
	PhoneNumber string `json:"phoneNumber"`
	Channel     string `json:"channel"`
}

type DNOQueryResponse struct {
	PhoneNumber string     `json:"phoneNumber"`
	IsDNO       bool       `json:"isDno"`
	Dataset     string     `json:"dataset,omitempty"`
	Channel     string     `json:"channel"`
	Status      string     `json:"status,omitempty"`
	LastUpdated *time.Time `json:"lastUpdated,omitempty"`
}

type BulkQueryRequest struct {
	PhoneNumbers []string `json:"phoneNumbers"`
	Channel      string   `json:"channel"`
}

type BulkQueryResponse struct {
	Results []DNOQueryResponse `json:"results"`
	Total   int                `json:"total"`
	Hits    int                `json:"hits"`
	Misses  int                `json:"misses"`
}

type AddDNORequest struct {
	PhoneNumber string `json:"phoneNumber"`
	NumberType  string `json:"numberType"`
	Channel     string `json:"channel"`
	Reason      string `json:"reason,omitempty"`
}

type ITGIngestRequest struct {
	PhoneNumber     string `json:"phoneNumber"`
	InvestigationID string `json:"investigationId"`
	ThreatCategory  string `json:"threatCategory"`
	Channel         string `json:"channel"`
}

type WebhookSubscription struct {
	ID        int64     `json:"id"`
	OrgID     int64     `json:"orgId"`
	URL       string    `json:"url"`
	Secret    string    `json:"secret,omitempty"`
	Events    string    `json:"events"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
}

type WebhookEvent struct {
	Event       string     `json:"event"`
	PhoneNumber string     `json:"phoneNumber"`
	Dataset     string     `json:"dataset"`
	Channel     string     `json:"channel"`
	Timestamp   time.Time  `json:"timestamp"`
	OrgID       *int64     `json:"orgId,omitempty"`
}

type NumberRegistryEntry struct {
	PhoneNumber string `json:"phoneNumber"`
	OwnerOrgID  *int64 `json:"ownerOrgId,omitempty"`
	NumberType  string `json:"numberType"`
	Status      string `json:"status"`
	TextEnabled bool   `json:"textEnabled"`
}

type DNOAnalyzerRequest struct {
	Records []CDRRecord `json:"records"`
	Channel string      `json:"channel"`
}

type CDRRecord struct {
	CallerID  string `json:"callerId"`
	Timestamp string `json:"timestamp,omitempty"`
}

type DNOAnalyzerReport struct {
	TotalRecords   int                `json:"totalRecords"`
	DNOHits        int                `json:"dnoHits"`
	DNOMisses      int                `json:"dnoMisses"`
	HitRate        float64            `json:"hitRate"`
	ByDataset      map[string]int     `json:"byDataset"`
	ByThreat       map[string]int     `json:"byThreatCategory"`
	TopSpoofed     []SpoofedNumber    `json:"topSpoofed"`
	EstBlockedPerDay int              `json:"estBlockedPerDay"`
}

type SpoofedNumber struct {
	PhoneNumber    string `json:"phoneNumber"`
	Dataset        string `json:"dataset"`
	ThreatCategory string `json:"threatCategory,omitempty"`
	Count          int    `json:"count"`
}

type ComplianceReport struct {
	GeneratedAt       time.Time      `json:"generatedAt"`
	OrgName           string         `json:"orgName,omitempty"`
	TotalDNONumbers   int            `json:"totalDnoNumbers"`
	DatasetCoverage   map[string]int `json:"datasetCoverage"`
	ChannelCoverage   map[string]int `json:"channelCoverage"`
	UpdateFrequency   string         `json:"updateFrequency"`
	EnforcementMethod string         `json:"enforcementMethod"`
	Last30DaysQueries int            `json:"last30DaysQueries"`
	Last30DaysHitRate float64        `json:"last30DaysHitRate"`
	Last30DaysBlocked int            `json:"last30DaysBlocked"`
	ComplianceStatus  string         `json:"complianceStatus"`
	Recommendations   []string       `json:"recommendations"`
}

type ROICalculation struct {
	DailyCallVolume     int     `json:"dailyCallVolume"`
	EstHitRate          float64 `json:"estHitRate"`
	EstDailyBlocked     int     `json:"estDailyBlocked"`
	EstMonthlyBlocked   int     `json:"estMonthlyBlocked"`
	EstAnnualBlocked    int     `json:"estAnnualBlocked"`
	AvgComplaintCost    float64 `json:"avgComplaintCost"`
	EstAnnualSavings    float64 `json:"estAnnualSavings"`
	ComplianceRiskLevel string  `json:"complianceRiskLevel"`
}

type AnalyticsSummary struct {
	TotalDNONumbers int            `json:"totalDnoNumbers"`
	ActiveNumbers   int            `json:"activeNumbers"`
	ByDataset       map[string]int `json:"byDataset"`
	ByChannel       map[string]int `json:"byChannel"`
	ByNumberType    map[string]int `json:"byNumberType"`
	TotalQueries24h int            `json:"totalQueries24h"`
	HitRate24h      float64        `json:"hitRate24h"`
	QueriesByHour   []HourlyCount  `json:"queriesByHour"`
	RecentAdditions int            `json:"recentAdditions"`
	RecentRemovals  int            `json:"recentRemovals"`
}

type HourlyCount struct {
	Hour  string `json:"hour"`
	Count int    `json:"count"`
}

type PaginatedResponse[T any] struct {
	Data       []T `json:"data"`
	Total      int `json:"total"`
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalPages int `json:"totalPages"`
}

type CreateUserRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Role      string `json:"role"`
	OrgID     *int64 `json:"orgId,omitempty"`
}

type CreateOrgRequest struct {
	Name      string  `json:"name"`
	OrgType   string  `json:"orgType"`
	SPID      *string `json:"spid,omitempty"`
	RespOrgID *string `json:"respOrgId,omitempty"`
}

// Validation helpers

var validChannels = map[string]bool{"voice": true, "text": true, "both": true}
var validDatasets = map[string]bool{"auto": true, "subscriber": true, "itg": true, "tss_registry": true}
var validStatuses = map[string]bool{"active": true, "inactive": true, "pending": true}
var validNumberTypes = map[string]bool{"toll_free": true, "local": true}
var validRoles = map[string]bool{"admin": true, "org_admin": true, "operator": true, "viewer": true}

func ValidateChannel(ch string) error {
	if !validChannels[ch] {
		return fmt.Errorf("invalid channel %q (valid: voice, text, both)", ch)
	}
	return nil
}

func ValidateDataset(ds string) error {
	if !validDatasets[ds] {
		return fmt.Errorf("invalid dataset %q (valid: auto, subscriber, itg, tss_registry)", ds)
	}
	return nil
}

func ValidateStatus(s string) error {
	if !validStatuses[s] {
		return fmt.Errorf("invalid status %q (valid: active, inactive, pending)", s)
	}
	return nil
}

func ValidateNumberType(nt string) error {
	if !validNumberTypes[nt] {
		return fmt.Errorf("invalid numberType %q (valid: toll_free, local)", nt)
	}
	return nil
}

func ValidateRole(r string) error {
	if !validRoles[r] {
		return fmt.Errorf("invalid role %q (valid: admin, org_admin, operator, viewer)", r)
	}
	return nil
}
