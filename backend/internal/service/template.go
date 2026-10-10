package service

import (
	"context"
	"fmt"
	"lxdapi/internal/db"
	"lxdapi/internal/lxc"
	"lxdapi/models"
	"lxdapi/pkg/logger"
)

type TemplateService struct {
	lxcClient *lxc.Client
}

func NewTemplateService() *TemplateService {
	return &TemplateService{
		lxcClient: lxc.NewClient(),
	}
}

func (s *TemplateService) List() ([]models.TemplateListResponse, error) {
	var templates []models.Template
	if err := db.DB.Order("created_at DESC").Find(&templates).Error; err != nil {
		return nil, err
	}
	
	result := make([]models.TemplateListResponse, len(templates))
	for i, t := range templates {
		result[i] = models.TemplateListResponse{
			Fingerprint:  t.Fingerprint,
			Alias:        t.Alias,
			Architecture: t.Architecture,
			Description:  t.Description,
			OS:           t.OS,
			Release:      t.Release,
			Size:         t.Size,
			SizeHuman:    formatSize(t.Size),
			Public:       t.Public,
			AutoUpdate:   t.AutoUpdate,
			UploadedAt:   t.UploadedAt,
			CreatedAt:    t.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}
	
	return result, nil
}

func (s *TemplateService) SyncFromLXD(ctx context.Context) (int, int, int, error) {
	logger.Info("开始从LXD同步镜像模板")
	
	images, err := s.lxcClient.ListImages(ctx)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("获取LXD镜像列表失败: %v", err)
	}
	
	added := 0
	updated := 0
	
	lxdFingerprints := make(map[string]bool)
	
	for _, img := range images {
		lxdFingerprints[img.Fingerprint] = true
		
		alias := ""
		if len(img.Aliases) > 0 {
			alias = img.Aliases[0].Name
		}
		
		os := ""
		release := ""
		description := ""
		
		if img.Properties != nil {
			if v, ok := img.Properties["os"].(string); ok {
				os = v
			}
			if v, ok := img.Properties["release"].(string); ok {
				release = v
			}
			if v, ok := img.Properties["description"].(string); ok {
				description = v
			}
		}
		
		var existing models.Template
		err := db.DB.Where("fingerprint = ?", img.Fingerprint).First(&existing).Error
		
		template := models.Template{
			Fingerprint:  img.Fingerprint,
			Alias:        alias,
			Architecture: img.Architecture,
			Description:  description,
			OS:           os,
			Release:      release,
			Size:         img.Size,
			Public:       img.Public,
			AutoUpdate:   img.AutoUpdate,
			UploadedAt:   img.UploadedAt,
		}
		
		if err != nil {
			if err := db.DB.Create(&template).Error; err != nil {
				logger.Warn("添加模板失败 %s: %v", img.Fingerprint, err)
				continue
			}
			added++
			logger.Info("添加模板: %s (%s)", alias, img.Fingerprint[:12])
		} else {
			template.ID = existing.ID
			if err := db.DB.Model(&existing).Updates(template).Error; err != nil {
				logger.Warn("更新模板失败 %s: %v", img.Fingerprint, err)
				continue
			}
			updated++
			logger.Info("更新模板: %s (%s)", alias, img.Fingerprint[:12])
		}
	}
	
	var dbTemplates []models.Template
	db.DB.Find(&dbTemplates)
	
	deleted := 0
	for _, dbTpl := range dbTemplates {
		if !lxdFingerprints[dbTpl.Fingerprint] {
			if err := db.DB.Unscoped().Delete(&dbTpl).Error; err != nil {
				logger.Warn("删除过期模板失败 %s: %v", dbTpl.Fingerprint, err)
				continue
			}
			deleted++
			logger.Info("删除过期模板: %s (%s)", dbTpl.Alias, dbTpl.Fingerprint[:12])
		}
	}
	
	logger.OK("同步完成: 新增 %d, 更新 %d, 删除 %d", added, updated, deleted)
	return added, updated, deleted, nil
}

func (s *TemplateService) Delete(ctx context.Context, fingerprint string) error {
	logger.Info("删除模板: %s", fingerprint)
	
	if err := s.lxcClient.DeleteImage(ctx, fingerprint); err != nil {
		return fmt.Errorf("从LXD删除镜像失败: %v", err)
	}
	
	if err := db.DB.Unscoped().Where("fingerprint = ?", fingerprint).Delete(&models.Template{}).Error; err != nil {
		logger.Warn("从数据库删除模板记录失败: %v", err)
	}
	
	logger.OK("模板删除成功: %s", fingerprint)
	return nil
}

func (s *TemplateService) BatchDelete(ctx context.Context, fingerprints []string) (int, int, error) {
	logger.Info("开始批量删除模板，数量: %d", len(fingerprints))
	
	success := 0
	failed := 0
	
	for _, fp := range fingerprints {
		if err := s.Delete(ctx, fp); err != nil {
			logger.Error("删除模板失败 %s: %v", fp, err)
			failed++
		} else {
			success++
		}
	}
	
	logger.OK("批量删除完成: 成功 %d, 失败 %d", success, failed)
	return success, failed, nil
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ImageOption 创建容器/商品时可选的系统镜像
type ImageOption struct {
	Value  string `json:"value"`  // 传给 lxc init 的镜像引用（本地别名/指纹，或 remote:alias）
	Label  string `json:"label"`  // 展示名称
	Arch   string `json:"arch"`   // 架构
	Source string `json:"source"` // local=本地已缓存镜像 / remote=远程镜像（首次使用需下载）
}

// RemoteImagePresets 常用远程镜像（LXD 默认远程：ubuntu: 为 Canonical 官方云镜像，images: 为社区镜像；
// images: 已不再提供 Ubuntu）
var RemoteImagePresets = []ImageOption{
	{Value: "ubuntu:24.04", Label: "Ubuntu 24.04 LTS", Source: "remote"},
	{Value: "ubuntu:22.04", Label: "Ubuntu 22.04 LTS", Source: "remote"},
	{Value: "images:debian/13", Label: "Debian 13", Source: "remote"},
	{Value: "images:debian/12", Label: "Debian 12", Source: "remote"},
	{Value: "images:rockylinux/9", Label: "Rocky Linux 9", Source: "remote"},
	{Value: "images:almalinux/9", Label: "AlmaLinux 9", Source: "remote"},
	{Value: "images:centos/9-Stream", Label: "CentOS Stream 9", Source: "remote"},
	{Value: "images:alpine/3.22", Label: "Alpine 3.22", Source: "remote"},
}

// templateImageRef 模板对应的 lxc init 镜像引用：优先别名，没有别名用指纹
func templateImageRef(t models.Template) string {
	if t.Alias != "" {
		return t.Alias
	}
	return t.Fingerprint
}

func templateLabel(t models.Template) string {
	label := t.Alias
	if t.Description != "" {
		if label != "" {
			label += " - " + t.Description
		} else {
			label = t.Description
		}
	}
	if label == "" {
		label = t.OS + " " + t.Release
	}
	if label == " " || label == "" {
		fp := t.Fingerprint
		if len(fp) > 12 {
			fp = fp[:12]
		}
		label = fp
	}
	return label
}

// ImageOptions 返回可选镜像：数据库模板（为空时自动从 LXD 同步一次）+ 常用远程镜像
func (s *TemplateService) ImageOptions(ctx context.Context) []ImageOption {
	var templates []models.Template
	db.DB.Order("created_at DESC").Find(&templates)
	if len(templates) == 0 {
		if _, _, _, err := s.SyncFromLXD(ctx); err != nil {
			logger.Warn("自动同步镜像模板失败: %v", err)
		} else {
			db.DB.Order("created_at DESC").Find(&templates)
		}
	}
	opts := make([]ImageOption, 0, len(templates)+len(RemoteImagePresets))
	seen := map[string]bool{}
	for _, t := range templates {
		ref := templateImageRef(t)
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		opts = append(opts, ImageOption{Value: ref, Label: templateLabel(t), Arch: t.Architecture, Source: "local"})
	}
	for _, r := range RemoteImagePresets {
		if !seen[r.Value] {
			opts = append(opts, r)
		}
	}
	return opts
}

// DefaultImage 商品未配置镜像时使用的默认镜像：第一个本地模板，否则 ubuntu:24.04
func DefaultImage() string {
	var t models.Template
	if err := db.DB.Order("created_at DESC").First(&t).Error; err == nil {
		if ref := templateImageRef(t); ref != "" {
			return ref
		}
	}
	return RemoteImagePresets[0].Value
}
