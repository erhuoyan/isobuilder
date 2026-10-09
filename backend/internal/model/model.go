// Package model holds the domain types shared by the store, builder and HTTP layers.
package model

// Architecture values.
const (
	ArchX86 = "x86_64"
	ArchARM = "aarch64"
)

// ISO status values.
const (
	IsoUploading   = "uploading"
	IsoDownloading = "downloading"
	IsoHashing     = "hashing"
	IsoReady       = "ready"
	IsoError       = "error"
)

// ISO sources.
const (
	SourceUploaded   = "uploaded"
	SourceRegistered = "registered"
	SourceS3         = "s3"
)

// Build status values.
const (
	BuildQueued    = "queued"
	BuildRunning   = "running"
	BuildSucceeded = "succeeded"
	BuildFailed    = "failed"
	BuildCanceled  = "canceled"
)

// Build stage identifiers, in execution order.
var Stages = []string{"mount", "inject", "boot", "build", "seal"}

// StageProgress maps a 1-based stage to the progress range it covers within 0-100.
func StageProgress(index int) (start, end int) {
	switch index {
	case 1:
		return 5, 20
	case 2:
		return 20, 35
	case 3:
		return 35, 50
	case 4:
		return 50, 90
	case 5:
		return 90, 99
	default:
		return 0, 0
	}
}

// ISO is a base image available for building.
type ISO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Filename  string `json:"filename"`
	Arch      string `json:"arch"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
	Status    string `json:"status"`
	Source    string `json:"source"`
	Path      string `json:"path"`
	Error     string `json:"error"`
	CreatedAt string `json:"created_at"`
}

// Kickstart is a kickstart template.
type Kickstart struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Arch         string   `json:"arch"`
	Builtin      bool     `json:"builtin"`
	Placeholders []string `json:"placeholders"`
	SizeBytes    int64    `json:"size_bytes"`
	Path         string   `json:"-"`
	CreatedAt    string   `json:"created_at"`
}

// HostSummary is one row of the parsed host inventory.
type HostSummary struct {
	SN          string `json:"sn"`
	Hostname    string `json:"hostname"`
	IP          string `json:"ip"`
	NetworkMode string `json:"network_mode"`
	MACCount    int    `json:"mac_count"`
	BMCIP       string `json:"bmc_ip"`
}

// Project is a buildable configuration.
type Project struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Arch         string        `json:"arch"`
	IsoID        string        `json:"iso_id"`
	Iso          *ISO          `json:"iso"`
	KickstartID  string        `json:"kickstart_id"`
	Kickstart    *Kickstart    `json:"kickstart"`
	IsoLabel     string        `json:"iso_label"`
	OutputName   string        `json:"output_name"`
	HostsFormat  string        `json:"hosts_format"`
	HostsRaw     string        `json:"hosts_raw,omitempty"`
	HostsJSON    string        `json:"-"`
	HostsCount   int           `json:"hosts_count"`
	HostsSummary []HostSummary `json:"hosts_summary"`
	CreatedAt    string        `json:"created_at"`
	UpdatedAt    string        `json:"updated_at"`
}

// Build is one ISO build job.
type Build struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	ProjectName string    `json:"project_name"`
	Status      string    `json:"status"`
	Stage       string    `json:"stage"`
	StageIndex  int       `json:"stage_index"`
	StageTotal  int       `json:"stage_total"`
	Progress    int       `json:"progress"`
	Arch        string    `json:"arch"`
	IsoName     string    `json:"iso_name"`
	IsoLabel    string    `json:"iso_label"`
	OutputName  string    `json:"output_name"`
	HostsCount  int       `json:"hosts_count"`
	LogBytes    int64     `json:"log_bytes"`
	Error       string    `json:"error"`
	Artifact    *Artifact `json:"artifact"`
	CreatedAt   string    `json:"created_at"`
	StartedAt   string    `json:"started_at,omitempty"`
	FinishedAt  string    `json:"finished_at,omitempty"`

	WorkDir     string `json:"-"`
	IsoPath     string `json:"-"`
	IsoArch     string `json:"-"`
	KSID        string `json:"-"`
	KSPath      string `json:"-"`
	HostsJSON   string `json:"-"`
	ArtifactID  string `json:"-"`
	Description string `json:"-"`
}

// Artifact is a produced auto-install ISO.
type Artifact struct {
	ID           string `json:"id"`
	BuildID      string `json:"build_id"`
	Filename     string `json:"filename"`
	SizeBytes    int64  `json:"size_bytes"`
	SHA256       string `json:"sha256"`
	Storage      string `json:"storage"`
	StorageKey   string `json:"storage_key,omitempty"`
	URL          string `json:"url"`
	URLExpiresAt string `json:"url_expires_at,omitempty"`
	LocalPath    string `json:"-"`
	CreatedAt    string `json:"created_at"`
}

// Terminal reports whether a build status is final.
func Terminal(status string) bool {
	return status == BuildSucceeded || status == BuildFailed || status == BuildCanceled
}
