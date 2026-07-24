// Package backup owns offline backup bundle validation and maintenance-aware
// backup/restore orchestration. It never exposes backup contents through HTTP.
package backup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DatabaseDumpName  = "database.dump"
	FilesArchiveName  = "files.tar.zst"
	ManifestName      = "manifest.json"
	ChecksumsName     = "checksums.sha256"
	bundleNamePrefix  = "backup-"
	checksumsFileMode = 0o600
)

// Manifest describes a portable backup without storing source paths, secrets,
// study material, prompts, or raw provider responses.
type Manifest struct {
	FormatVersion   int       `json:"format_version"`
	BackupID        string    `json:"backup_id"`
	AppVersion      string    `json:"app_version"`
	DatabaseVersion string    `json:"database_version"`
	CreatedAt       time.Time `json:"created_at"`
	FileCount       int64     `json:"file_count"`
	FileBytes       int64     `json:"file_bytes"`
	ActiveJobs      int64     `json:"active_jobs"`
}

// WriteBundleManifest writes manifest.json followed by the checksum inventory.
// The caller is responsible for atomically publishing the containing directory.
func WriteBundleManifest(dir string, manifest Manifest) (string, error) {
	if manifest.FormatVersion != 1 {
		return "", errors.New("不支持的备份包格式")
	}
	if manifest.BackupID == "" {
		return "", errors.New("备份包缺少备份 ID")
	}
	if manifest.CreatedAt.IsZero() {
		return "", errors.New("备份包缺少创建时间")
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("编码备份清单失败: %w", err)
	}
	body = append(body, '\n')
	if err := os.WriteFile(filepath.Join(dir, ManifestName), body, checksumsFileMode); err != nil {
		return "", fmt.Errorf("写入备份清单失败: %w", err)
	}

	var lines strings.Builder
	for _, name := range requiredBundleFiles {
		sum, err := hashRegularFile(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&lines, "%s  %s\n", sum, name)
	}
	checksum := sha256.Sum256([]byte(lines.String()))
	if err := os.WriteFile(filepath.Join(dir, ChecksumsName), []byte(lines.String()), checksumsFileMode); err != nil {
		return "", fmt.Errorf("写入备份校验清单失败: %w", err)
	}
	return hex.EncodeToString(checksum[:]), nil
}

var requiredBundleFiles = []string{DatabaseDumpName, FilesArchiveName, ManifestName}

// VerifyBundle validates a locally selected backup bundle before restore.
func VerifyBundle(root, name string) (Manifest, string, error) {
	if !validBundleName(name) {
		return Manifest{}, "", errors.New("备份包名称非法")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Manifest{}, "", errors.New("备份目录无效")
	}
	dir := filepath.Join(absRoot, name)
	if filepath.Dir(dir) != absRoot {
		return Manifest{}, "", errors.New("备份包路径非法")
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return Manifest{}, "", errors.New("备份包目录无效")
	}

	checksums, err := readChecksums(filepath.Join(dir, ChecksumsName))
	if err != nil {
		return Manifest{}, "", err
	}
	for _, file := range requiredBundleFiles {
		got, err := hashRegularFile(filepath.Join(dir, file))
		if err != nil {
			return Manifest{}, "", err
		}
		if want, ok := checksums[file]; !ok || got != want {
			return Manifest{}, "", errors.New("备份包校验失败")
		}
	}

	body, err := os.ReadFile(filepath.Join(dir, ManifestName))
	if err != nil {
		return Manifest{}, "", errors.New("读取备份清单失败")
	}
	var manifest Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return Manifest{}, "", errors.New("备份清单格式无效")
	}
	if manifest.FormatVersion != 1 || manifest.BackupID == "" || manifest.CreatedAt.IsZero() {
		return Manifest{}, "", errors.New("备份清单内容无效")
	}
	return manifest, dir, nil
}

func validBundleName(name string) bool {
	return strings.HasPrefix(name, bundleNamePrefix) && len(name) > len(bundleNamePrefix) &&
		filepath.Base(name) == name && !strings.ContainsAny(name, `\\/`) && name != "." && name != ".."
}

func readChecksums(path string) (map[string]string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("读取备份校验清单失败")
	}
	checksums := make(map[string]string, len(requiredBundleFiles))
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		parts := strings.Fields(line)
		if len(parts) != 2 || len(parts[0]) != 64 {
			return nil, errors.New("备份校验清单格式无效")
		}
		if _, err := hex.DecodeString(parts[0]); err != nil {
			return nil, errors.New("备份校验清单格式无效")
		}
		if _, exists := checksums[parts[1]]; exists {
			return nil, errors.New("备份校验清单包含重复条目")
		}
		checksums[parts[1]] = parts[0]
	}
	if len(checksums) != len(requiredBundleFiles) {
		return nil, errors.New("备份校验清单不完整")
	}
	return checksums, nil
}

func hashRegularFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("备份包缺少必需文件")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", errors.New("读取备份包文件失败")
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", errors.New("计算备份文件校验和失败")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
