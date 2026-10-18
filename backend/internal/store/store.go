package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"isobuilder/internal/model"
)

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Now is the canonical timestamp written to every row.
func Now() string { return time.Now().UTC().Format(time.RFC3339) }

// NewID returns a fresh 12 character hex identifier.
func NewID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%012x", time.Now().UnixNano()&0xffffffffffff)
	}
	return hex.EncodeToString(b[:])
}

// --- ISO ----------------------------------------------------------------

// ListISOs returns every ISO record, newest first.
func (s *Store) ListISOs() ([]model.ISO, error) {
	rows, err := s.query(`SELECT id,name,filename,arch,size_bytes,sha256,md5,remark,status,source,path,error,created_at
	                        FROM isos ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.ISO{}
	for rows.Next() {
		var i model.ISO
		if err := rows.Scan(&i.ID, &i.Name, &i.Filename, &i.Arch, &i.SizeBytes, &i.SHA256, &i.MD5, &i.Remark,
			&i.Status, &i.Source, &i.Path, &i.Error, &i.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// GetISO returns one ISO record.
func (s *Store) GetISO(id string) (model.ISO, error) {
	var i model.ISO
	err := s.queryRow(`SELECT id,name,filename,arch,size_bytes,sha256,md5,remark,status,source,path,error,created_at
	                      FROM isos WHERE id=?`, id).
		Scan(&i.ID, &i.Name, &i.Filename, &i.Arch, &i.SizeBytes, &i.SHA256, &i.MD5, &i.Remark, &i.Status, &i.Source, &i.Path, &i.Error, &i.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return i, ErrNotFound
	}
	return i, err
}

// CreateISO inserts an ISO record.
func (s *Store) CreateISO(i model.ISO) error {
	_, err := s.exec(`INSERT INTO isos(id,name,filename,arch,size_bytes,sha256,md5,remark,status,source,path,error,created_at)
	                     VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, i.ID, i.Name, i.Filename, i.Arch, i.SizeBytes, i.SHA256, i.MD5, i.Remark, i.Status, i.Source, i.Path, i.Error, i.CreatedAt)
	return err
}

// FinishISO records the terminal state of an upload or hash probe.
func (s *Store) FinishISO(id, status, sha256 string, sizeBytes int64, errMsg string) error {
	return s.FinishISOWithMD5(id, status, sha256, "", sizeBytes, errMsg)
}

// FinishISOWithMD5 additionally records an MD5 digest (装机校验习惯用 MD5).
func (s *Store) FinishISOWithMD5(id, status, sha256, md5 string, sizeBytes int64, errMsg string) error {
	_, err := s.exec(`UPDATE isos SET status=?, sha256=?, md5=?, size_bytes=?, error=? WHERE id=?`, status, sha256, md5, sizeBytes, errMsg, id)
	return err
}

// DeleteISO removes an ISO record.
func (s *Store) DeleteISO(id string) error {
	_, err := s.exec(`DELETE FROM isos WHERE id=?`, id)
	return err
}

// --- Kickstart ----------------------------------------------------------

// ListKickstarts returns templates, built-ins first.
func (s *Store) ListKickstarts() ([]model.Kickstart, error) {
	rows, err := s.query(`SELECT id,name,arch,builtin,remark,path,size_bytes,created_at FROM kickstarts
	                         ORDER BY builtin DESC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Kickstart{}
	for rows.Next() {
		var k model.Kickstart
		if err := rows.Scan(&k.ID, &k.Name, &k.Arch, &k.Builtin, &k.Remark, &k.Path, &k.SizeBytes, &k.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// GetKickstart returns one template.
func (s *Store) GetKickstart(id string) (model.Kickstart, error) {
	var k model.Kickstart
	err := s.queryRow(`SELECT id,name,arch,builtin,remark,path,size_bytes,created_at FROM kickstarts WHERE id=?`, id).
		Scan(&k.ID, &k.Name, &k.Arch, &k.Builtin, &k.Remark, &k.Path, &k.SizeBytes, &k.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return k, ErrNotFound
	}
	return k, err
}

// CreateKickstart inserts a template record.
func (s *Store) CreateKickstart(k model.Kickstart) error {
	_, err := s.exec(`INSERT INTO kickstarts(id,name,arch,builtin,remark,path,size_bytes,created_at) VALUES(?,?,?,?,?,?,?,?)`, k.ID, k.Name, k.Arch, k.Builtin, k.Remark, k.Path, k.SizeBytes, k.CreatedAt)
	return err
}

// UpdateKickstart updates the editable fields of a template.
func (s *Store) UpdateKickstart(id, name, remark string) error {
	_, err := s.exec(`UPDATE kickstarts SET name=?, remark=? WHERE id=?`, name, remark, id)
	return err
}

// DeleteKickstart removes a template record.
func (s *Store) DeleteKickstart(id string) error {
	_, err := s.exec(`DELETE FROM kickstarts WHERE id=?`, id)
	return err
}

// --- Script（装机后脚本库）-----------------------------------------------

// ListScripts returns every reusable post-install script, newest edit first.
func (s *Store) ListScripts() ([]model.Script, error) {
	rows, err := s.query(`SELECT id,name,remark,content,size_bytes,created_at,updated_at
	                     FROM scripts ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Script{}
	for rows.Next() {
		var sc model.Script
		if err := rows.Scan(&sc.ID, &sc.Name, &sc.Remark, &sc.Content, &sc.SizeBytes, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

// GetScript returns one script, including its body.
func (s *Store) GetScript(id string) (model.Script, error) {
	var sc model.Script
	err := s.queryRow(`SELECT id,name,remark,content,size_bytes,created_at,updated_at FROM scripts WHERE id=?`, id).
		Scan(&sc.ID, &sc.Name, &sc.Remark, &sc.Content, &sc.SizeBytes, &sc.CreatedAt, &sc.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return sc, ErrNotFound
	}
	return sc, err
}

// CreateScript inserts a script record.
func (s *Store) CreateScript(sc model.Script) error {
	_, err := s.exec(`INSERT INTO scripts(id,name,remark,content,size_bytes,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`,
		sc.ID, sc.Name, sc.Remark, sc.Content, sc.SizeBytes, sc.CreatedAt, sc.UpdatedAt)
	return err
}

// UpdateScript replaces the editable fields and restamps updated_at.
func (s *Store) UpdateScript(id, name, remark, content string) error {
	_, err := s.exec(`UPDATE scripts SET name=?, remark=?, content=?, size_bytes=?, updated_at=? WHERE id=?`,
		name, remark, content, len(content), Now(), id)
	return err
}

// DeleteScript removes a script record.
func (s *Store) DeleteScript(id string) error {
	_, err := s.exec(`DELETE FROM scripts WHERE id=?`, id)
	return err
}

// CountProjectsUsingScript counts projects bound to a script.
func (s *Store) CountProjectsUsingScript(id string) (int, error) {
	return s.count(`SELECT COUNT(*) FROM projects WHERE script_id=?`, id)
}

// --- Project ------------------------------------------------------------

const projectCols = `id,name,description,arch,iso_id,kickstart_id,script_id,iso_label,output_name,post_script,payload_mode,hosts_format,hosts_raw,hosts_json,hosts_count,hosts_summary,created_at,updated_at`

type scanner interface{ Scan(dest ...any) error }

func scanProject(sc scanner) (model.Project, error) {
	var p model.Project
	var summary string
	err := sc.Scan(&p.ID, &p.Name, &p.Description, &p.Arch, &p.IsoID, &p.KickstartID, &p.ScriptID, &p.IsoLabel,
		&p.OutputName, &p.PostScript, &p.PayloadMode, &p.HostsFormat, &p.HostsRaw, &p.HostsJSON, &p.HostsCount, &summary, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return p, err
	}
	if summary != "" && summary != "null" {
		if err := json.Unmarshal([]byte(summary), &p.HostsSummary); err != nil {
			return p, fmt.Errorf("decode hosts_summary: %w", err)
		}
	}
	if p.HostsSummary == nil {
		p.HostsSummary = []model.HostSummary{}
	}
	return p, nil
}

// ListProjects returns every project without the raw inventory body, with the
// referenced ISO and kickstart inlined for display.
func (s *Store) ListProjects() ([]model.Project, error) {
	rows, err := s.query(`SELECT ` + projectCols + ` FROM projects ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Project{}
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, s.inlineRefs(out)
}

// GetProject returns one project including its raw inventory.
func (s *Store) GetProject(id string) (model.Project, error) {
	p, err := scanProject(s.queryRow(`SELECT `+projectCols+` FROM projects WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	if err != nil {
		return p, err
	}
	list := []model.Project{p}
	if err := s.inlineRefs(list); err != nil {
		return p, err
	}
	return list[0], nil
}

// inlineRefs fills in the referenced ISO and kickstart. A dangling reference
// yields nil rather than an error, because referenced rows can be deleted.
func (s *Store) inlineRefs(projects []model.Project) error {
	if len(projects) == 0 {
		return nil
	}
	isos, err := s.ListISOs()
	if err != nil {
		return err
	}
	ksList, err := s.ListKickstarts()
	if err != nil {
		return err
	}
	isoByID := make(map[string]model.ISO, len(isos))
	for _, i := range isos {
		isoByID[i.ID] = i
	}
	scriptsList, err := s.ListScripts()
	if err != nil {
		return err
	}
	ksByID := make(map[string]model.Kickstart, len(ksList))
	for _, k := range ksList {
		ksByID[k.ID] = k
	}
	scriptByID := make(map[string]model.Script, len(scriptsList))
	for _, sc := range scriptsList {
		scriptByID[sc.ID] = sc
	}
	for i := range projects {
		if iso, ok := isoByID[projects[i].IsoID]; ok {
			cp := iso
			projects[i].Iso = &cp
		}
		if ks, ok := ksByID[projects[i].KickstartID]; ok {
			cp := ks
			projects[i].Kickstart = &cp
		}
		if sc, ok := scriptByID[projects[i].ScriptID]; ok {
			cp := sc
			cp.Content = "" // 列表只需元信息，正文按需单独查，避免响应膨胀
			projects[i].Script = &cp
		}
	}
	return nil
}

// SaveProject inserts or replaces a project.
func (s *Store) SaveProject(p model.Project, insert bool) error {
	summary, err := json.Marshal(p.HostsSummary)
	if err != nil {
		return err
	}
	if insert {
		_, err = s.exec(`INSERT INTO projects(`+projectCols+`) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			p.ID, p.Name, p.Description, p.Arch, p.IsoID, p.KickstartID, p.ScriptID, p.IsoLabel, p.OutputName,
			p.PostScript, p.PayloadMode, p.HostsFormat, p.HostsRaw, p.HostsJSON, p.HostsCount, string(summary), p.CreatedAt, p.UpdatedAt)
		return err
	}
	_, err = s.exec(`UPDATE projects SET name=?, description=?, arch=?, iso_id=?, kickstart_id=?, script_id=?,
	                    iso_label=?, output_name=?, post_script=?, payload_mode=?, hosts_format=?, hosts_raw=?, hosts_json=?,
	                    hosts_count=?, hosts_summary=?, updated_at=? WHERE id=?`,
		p.Name, p.Description, p.Arch, p.IsoID, p.KickstartID, p.ScriptID, p.IsoLabel, p.OutputName,
		p.PostScript, p.PayloadMode, p.HostsFormat, p.HostsRaw, p.HostsJSON, p.HostsCount, string(summary), p.UpdatedAt, p.ID)
	return err
}

// UpdateProjectSummary refreshes only hosts_count/hosts_summary. It exists for
// the startup backfill and deliberately leaves updated_at untouched: a derived
// data refresh is not a user edit and must not re-order the project list.
func (s *Store) UpdateProjectSummary(id string, count int, summary []model.HostSummary) error {
	body, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	_, err = s.exec(`UPDATE projects SET hosts_count=?, hosts_summary=? WHERE id=?`, count, string(body), id)
	return err
}

// DeleteProject removes a project together with its build history.
func (s *Store) DeleteProject(id string) error {
	if _, err := s.exec(`DELETE FROM builds WHERE project_id=?`, id); err != nil {
		return err
	}
	_, err := s.exec(`DELETE FROM projects WHERE id=?`, id)
	return err
}

// CountProjectsUsingISO counts projects bound to an ISO.
func (s *Store) CountProjectsUsingISO(id string) (int, error) {
	return s.count(`SELECT COUNT(*) FROM projects WHERE iso_id=?`, id)
}

// CountProjectsUsingKickstart counts projects bound to a template.
func (s *Store) CountProjectsUsingKickstart(id string) (int, error) {
	return s.count(`SELECT COUNT(*) FROM projects WHERE kickstart_id=?`, id)
}

func (s *Store) count(query string, args ...any) (int, error) {
	var n int
	err := s.queryRow(query, args...).Scan(&n)
	return n, err
}

// --- Build --------------------------------------------------------------

const buildCols = `id,project_id,project_name,status,stage,stage_index,stage_total,progress,arch,iso_id,iso_name,
iso_label,kickstart_id,hosts_json,hosts_count,output_name,post_script,payload_mode,work_dir,error,artifact_id,created_at,started_at,finished_at`

func scanBuild(sc scanner) (model.Build, error) {
	var b model.Build
	err := sc.Scan(&b.ID, &b.ProjectID, &b.ProjectName, &b.Status, &b.Stage, &b.StageIndex, &b.StageTotal,
		&b.Progress, &b.Arch, &b.IsoPath, &b.IsoName, &b.IsoLabel, &b.KSID, &b.HostsJSON, &b.HostsCount,
		&b.OutputName, &b.PostScript, &b.PayloadMode, &b.WorkDir, &b.Error, &b.ArtifactID, &b.CreatedAt, &b.StartedAt, &b.FinishedAt)
	if err != nil {
		return b, err
	}
	b.IsoArch = b.Arch
	b.KSPath = b.KSID
	return b, nil
}

// CreateBuild inserts a queued build. The build row snapshots everything the
// worker needs so a project edit mid-build cannot change the outcome.
func (s *Store) CreateBuild(b model.Build) error {
	_, err := s.exec(`INSERT INTO builds(`+buildCols+`) VALUES(`+s.placeholders(24)+`)`, b.ID, b.ProjectID, b.ProjectName, b.Status, b.Stage, b.StageIndex, b.StageTotal, b.Progress,
		b.Arch, b.IsoPath, b.IsoName, b.IsoLabel, b.KSID, b.HostsJSON, b.HostsCount, b.OutputName,
		b.PostScript, b.PayloadMode, b.WorkDir, b.Error, b.ArtifactID, b.CreatedAt, b.StartedAt, b.FinishedAt)
	return err
}

func (s *Store) buildByID(id string) (model.Build, error) {
	b, err := scanBuild(s.queryRow(`SELECT `+buildCols+` FROM builds WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return b, ErrNotFound
	}
	return b, err
}

// GetBuild returns one build with its artifact inlined.
func (s *Store) GetBuild(id string) (model.Build, error) {
	b, err := s.buildByID(id)
	if err != nil {
		return b, err
	}
	return s.withArtifact(b)
}

// ListBuilds returns builds newest first, optionally scoped to one project.
func (s *Store) ListBuilds(projectID string, limit int) ([]model.Build, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	query := `SELECT ` + buildCols + ` FROM builds`
	args := []any{}
	if projectID != "" {
		query += ` WHERE project_id=?`
		args = append(args, projectID)
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Build{}
	for rows.Next() {
		b, err := scanBuild(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	artifacts, err := s.artifactByBuild()
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Artifact = artifacts[out[i].ID]
	}
	out = s.fillMissingSnapshot(out)
	return out, nil
}

// fillMissingSnapshot rehydrates the display fields of old build rows whose
// snapshot columns are empty.
func (s *Store) fillMissingSnapshot(builds []model.Build) []model.Build {
	for i := range builds {
		if builds[i].ProjectName != "" {
			continue
		}
		if p, err := s.GetProject(builds[i].ProjectID); err == nil {
			builds[i].ProjectName = p.Name
		}
	}
	return builds
}

func (s *Store) withArtifact(b model.Build) (model.Build, error) {
	if b.ArtifactID == "" {
		return b, nil
	}
	a, err := s.GetArtifact(b.ArtifactID)
	if err != nil {
		return b, nil
	}
	b.Artifact = &a
	return b, nil
}

// StartBuild marks a build running and stamps its start time.
func (s *Store) StartBuild(id string) error {
	_, err := s.exec(`UPDATE builds SET status=?, started_at=? WHERE id=?`, model.BuildRunning, Now(), id)
	return err
}

// SetBuildStage records the pipeline stage, its 1-based index and progress.
func (s *Store) SetBuildStage(id, stage string, index, progress int) error {
	_, err := s.exec(`UPDATE builds SET stage=?, stage_index=?, progress=? WHERE id=?`, stage, index, progress, id)
	return err
}

// SetBuildWorkDir records the scratch directory used by the build.
func (s *Store) SetBuildWorkDir(id, dir string) error {
	_, err := s.exec(`UPDATE builds SET work_dir=? WHERE id=?`, dir, id)
	return err
}

// FinishBuild marks a build terminal.
func (s *Store) FinishBuild(id, status, errMsg string, progress int) error {
	_, err := s.exec(`UPDATE builds SET status=?, error=?, progress=?, finished_at=? WHERE id=?`, status, errMsg, progress, Now(), id)
	return err
}

// SetBuildArtifact links the produced artifact to its build.
func (s *Store) SetBuildArtifact(buildID, artifactID string) error {
	_, err := s.exec(`UPDATE builds SET artifact_id=? WHERE id=?`, artifactID, buildID)
	return err
}

// ActiveBuildIDs lists queued and running builds, oldest first.
func (s *Store) ActiveBuildIDs() ([]string, error) {
	rows, err := s.query(`SELECT id FROM builds WHERE status IN (?,?) ORDER BY created_at, id`, model.BuildQueued, model.BuildRunning)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// CountActiveBuilds counts queued and running builds.
func (s *Store) CountActiveBuilds() (int, error) {
	return s.count(`SELECT COUNT(*) FROM builds WHERE status IN (?,?)`, model.BuildQueued, model.BuildRunning)
}

// CountActiveBuildsForProject counts queued and running builds of one project.
func (s *Store) CountActiveBuildsForProject(projectID string) (int, error) {
	return s.count(`SELECT COUNT(*) FROM builds WHERE project_id=? AND status IN (?,?)`,
		projectID, model.BuildQueued, model.BuildRunning)
}

// --- Artifact -----------------------------------------------------------

// CreateArtifact inserts an artifact record.
func (s *Store) CreateArtifact(a model.Artifact) error {
	_, err := s.exec(`INSERT INTO artifacts(id,build_id,filename,size_bytes,sha256,md5,storage,storage_key,url,url_expires_at,local_path,created_at)
	                     VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, a.ID, a.BuildID, a.Filename, a.SizeBytes, a.SHA256, a.MD5, a.Storage, a.StorageKey, a.URL,
		a.URLExpiresAt, a.LocalPath, a.CreatedAt)
	return err
}

// RefreshArtifactURL stores a freshly signed URL.
func (s *Store) RefreshArtifactURL(id, url, expiresAt string) error {
	_, err := s.exec(`UPDATE artifacts SET url=?, url_expires_at=? WHERE id=?`, url, expiresAt, id)
	return err
}

// GetArtifact returns one artifact.
func (s *Store) GetArtifact(id string) (model.Artifact, error) {
	var a model.Artifact
	err := s.queryRow(`SELECT id,build_id,filename,size_bytes,sha256,md5,storage,storage_key,url,url_expires_at,local_path,created_at
	                      FROM artifacts WHERE id=?`, id).
		Scan(&a.ID, &a.BuildID, &a.Filename, &a.SizeBytes, &a.SHA256, &a.MD5, &a.Storage, &a.StorageKey,
			&a.URL, &a.URLExpiresAt, &a.LocalPath, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

func (s *Store) artifactByBuild() (map[string]*model.Artifact, error) {
	rows, err := s.query(`SELECT id,build_id,filename,size_bytes,sha256,md5,storage,storage_key,url,url_expires_at,local_path,created_at
	                         FROM artifacts ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]*model.Artifact{}
	for rows.Next() {
		var a model.Artifact
		if err := rows.Scan(&a.ID, &a.BuildID, &a.Filename, &a.SizeBytes, &a.SHA256, &a.MD5, &a.Storage,
			&a.StorageKey, &a.URL, &a.URLExpiresAt, &a.LocalPath, &a.CreatedAt); err != nil {
			return nil, err
		}
		cp := a
		if _, seen := out[a.BuildID]; !seen {
			out[a.BuildID] = &cp
		}
	}
	return out, rows.Err()
}

// --- Settings -----------------------------------------------------------

// GetSetting returns a settings row, or an empty string when unset.
func (s *Store) GetSetting(key string) (string, error) {
	var value string
	err := s.queryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}

// SetSetting writes a settings row. The upsert syntax is the only statement
// that differs across dialects, so it is chosen here rather than abstracted
// away — everything else in this package is portable SQL.
func (s *Store) SetSetting(key, value string) error {
	var q string
	if s.dialect == "mysql" {
		q = `INSERT INTO settings(` + "`key`" + `, value, updated_at) VALUES(?,?,?)
		     ON DUPLICATE KEY UPDATE value=VALUES(value), updated_at=VALUES(updated_at)`
	} else {
		q = `INSERT INTO settings(key, value, updated_at) VALUES(?,?,?)
		     ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`
	}
	_, err := s.exec(q, key, value, Now())
	return err
}
