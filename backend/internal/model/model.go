// Package model holds the domain types shared by the store, builder and HTTP layers.
package model

// Architecture values.
const (
	ArchX86 = "x86_64"
	ArchARM = "aarch64"
)

// ISO status values.
const (
	IsoDownloading = "downloading"
	IsoHashing     = "hashing"
	IsoReady       = "ready"
	IsoError       = "error"
)

// ISO sources. Images enter the library either from the server's own disk
// (registered) or by the server downloading a link (url); the browser never
// uploads an image.
const (
	SourceRegistered = "registered"
	SourceURL        = "url"
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
	MD5       string `json:"md5"`
	Remark    string `json:"remark"`
	Status    string `json:"status"`
	Source    string `json:"source"`
	// Path is relative to data_dir (e.g. "isos/<id>-<name>.iso") — never
	// absolute. data_dir is a deployment setting that changes between hosts and
	// mount points; storing it inline orphaned every row whenever it moved.
	// The API still reports an absolute-looking path via store.ISOPath.
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
	Remark       string   `json:"remark"`
	Placeholders []string `json:"placeholders"`
	SizeBytes    int64    `json:"size_bytes"`
	Path         string   `json:"-"`
	CreatedAt    string   `json:"created_at"`
}

// Script is a reusable post-install script kept in its own library, so several
// projects can share one script instead of each pasting its own copy.
type Script struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Remark    string `json:"remark"`
	Content   string `json:"content"`
	SizeBytes int64  `json:"size_bytes"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// HostSummary is one row of the parsed host inventory.
type HostSummary struct {
	SN          string `json:"sn"`
	Hostname    string `json:"hostname"`
	IP          string `json:"ip"`
	Gateway     string `json:"gateway"`
	DiskGB      int    `json:"disk_gb"`
	NetworkMode string `json:"network_mode"`
	MACCount    int    `json:"mac_count"`
	// VLANID is the 802.1Q tag the address is configured on; 0 means untagged.
	VLANID int    `json:"vlan_id"`
	BMCIP  string `json:"bmc_ip"`
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
	ScriptID     string        `json:"script_id"`
	Script       *Script       `json:"script"`
	IsoLabel     string        `json:"iso_label"`
	OutputName   string        `json:"output_name"`
	PostScript   string        `json:"post_script"`
	PayloadMode  string        `json:"payload_mode"`
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
	PostScript  string `json:"-"`
	PayloadMode string `json:"-"`
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
	MD5          string `json:"md5"`
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
