package service

import (
	"lxdapi/internal/core"
	"lxdapi/internal/db"
	"lxdapi/models"
	"lxdapi/pkg/logger"
	"os"
	"path/filepath"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type BrandService struct{}

func NewBrandService() *BrandService {
	return &BrandService{}
}

func (s *BrandService) GetSettings() (*models.BrandSettings, error) {
	var settings models.BrandSettings
	
	if err := db.DB.First(&settings).Error; err != nil {
		settings = models.BrandSettings{
			AdminSystemName:      "LXD API - 管理后台",
			AdminSystemTitle:     "管理后台 - LXD容器管理系统",
			AdminLoginTitle:      "管理员登录",
			AdminBgImage:         "",
			AdminBgOpacity:       75,
			AdminContentOpacity:  85,
			UserSystemName:       "LXD API - 用户中心",
			UserSystemTitle:      "用户中心 - LXD容器管理系统",
			UserLoginTitle:       "用户登录",
			UserBgImage:          "",
			UserBgOpacity:        75,
			UserContentOpacity:   85,
			ContainerSystemName:  "LXD API - 容器控制",
			ContainerSystemTitle: "容器管理 - LXD容器管理系统",
			ContainerLoginTitle:  "容器登录",
			ContainerBgImage:     "",
			ContainerBgOpacity:   75,
			ContainerContentOpacity: 85,
			UserNotice:           "",
			ContainerNotice:      "",
			FaviconUrl:           "",
			FooterText:           "LXD API 容器管理平台",
		}
		
		if err := db.DB.Create(&settings).Error; err != nil {
			logger.Error("创建默认品牌设置失败: %v", err)
			return nil, err
		}
		logger.Info("已创建默认品牌设置")
	}
	
	return &settings, nil
}

func (s *BrandService) UpdateSettings(settings *models.BrandSettings) error {
	var existing models.BrandSettings
	
	if err := db.DB.First(&existing).Error; err != nil {
		if err := db.DB.Create(settings).Error; err != nil {
			logger.Error("创建品牌设置失败: %v", err)
			return err
		}
		logger.OK("品牌设置创建成功")
		return nil
	}
	
	existing.AdminSystemName = settings.AdminSystemName
	existing.AdminSystemTitle = settings.AdminSystemTitle
	existing.AdminLoginTitle = settings.AdminLoginTitle
	existing.AdminBgImage = settings.AdminBgImage
	existing.AdminBgOpacity = settings.AdminBgOpacity
	existing.AdminContentOpacity = settings.AdminContentOpacity
	existing.UserSystemName = settings.UserSystemName
	existing.UserSystemTitle = settings.UserSystemTitle
	existing.UserLoginTitle = settings.UserLoginTitle
	existing.UserBgImage = settings.UserBgImage
	existing.UserBgOpacity = settings.UserBgOpacity
	existing.UserContentOpacity = settings.UserContentOpacity
	existing.ContainerSystemName = settings.ContainerSystemName
	existing.ContainerSystemTitle = settings.ContainerSystemTitle
	existing.ContainerLoginTitle = settings.ContainerLoginTitle
	existing.ContainerBgImage = settings.ContainerBgImage
	existing.ContainerBgOpacity = settings.ContainerBgOpacity
	existing.ContainerContentOpacity = settings.ContainerContentOpacity
	existing.UserNotice = settings.UserNotice
	existing.UserNoticeOpacity = settings.UserNoticeOpacity
	existing.ContainerNotice = settings.ContainerNotice
	existing.ContainerNoticeOpacity = settings.ContainerNoticeOpacity
	existing.FaviconUrl = settings.FaviconUrl
	existing.FooterText = settings.FooterText
	existing.ContainerLiteTemplate = settings.ContainerLiteTemplate
	existing.ContainerBaseTemplate = settings.ContainerBaseTemplate
	existing.UseLocalCDN = settings.UseLocalCDN
	existing.TLSCertContent = settings.TLSCertContent
	existing.TLSKeyContent = settings.TLSKeyContent
	
	if err := db.DB.Save(&existing).Error; err != nil {
		logger.Error("更新品牌设置失败: %v", err)
		return err
	}
	
	if settings.TLSCertContent != "" && settings.TLSKeyContent != "" {
		if err := s.SaveTLSCertificates(settings.TLSCertContent, settings.TLSKeyContent); err != nil {
			logger.Error("保存TLS证书失败: %v", err)
			return err
		}
		logger.OK("TLS证书已更新")
	}
	
	logger.OK("品牌设置更新成功")
	return nil
}

func (s *BrandService) ResetToDefault() error {
	var settings models.BrandSettings
	
	if err := db.DB.First(&settings).Error; err != nil {
		return err
	}
	
	settings.AdminSystemName = "LXD API - 管理后台"
	settings.AdminSystemTitle = "管理后台 - LXD容器管理系统"
	settings.AdminLoginTitle = "管理员登录"
	settings.AdminBgImage = ""
	settings.AdminBgOpacity = 75
	settings.AdminContentOpacity = 85
	settings.UserSystemName = "LXD API - 用户中心"
	settings.UserSystemTitle = "用户中心 - LXD容器管理系统"
	settings.UserLoginTitle = "用户登录"
	settings.UserBgImage = ""
	settings.UserBgOpacity = 75
	settings.UserContentOpacity = 85
	settings.ContainerSystemName = "LXD API - 容器控制"
	settings.ContainerSystemTitle = "容器管理 - LXD容器管理系统"
	settings.ContainerLoginTitle = "容器登录"
	settings.ContainerBgImage = ""
	settings.ContainerBgOpacity = 75
	settings.ContainerContentOpacity = 85
	settings.UserNotice = ""
	settings.ContainerNotice = ""
	settings.FaviconUrl = ""
	settings.FooterText = "LXD API 容器管理平台"
	settings.TLSCertContent = ""
	settings.TLSKeyContent = ""
	
	if err := db.DB.Save(&settings).Error; err != nil {
		logger.Error("重置品牌设置失败: %v", err)
		return err
	}
	
	logger.OK("品牌设置已重置为默认值")
	return nil
}

func (s *BrandService) SaveTLSCertificates(certContent, keyContent string) error {
	cfg := core.GlobalConfig
	if cfg == nil {
		return nil
	}
	
	certFile := cfg.System.Server.TLS.CertFile
	keyFile := cfg.System.Server.TLS.KeyFile
	
	certDir := filepath.Dir(certFile)
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return err
	}
	
	keyDir := filepath.Dir(keyFile)
	if err := os.MkdirAll(keyDir, 0755); err != nil {
		return err
	}
	
	if err := os.WriteFile(certFile, []byte(certContent), 0644); err != nil {
		return err
	}
	
	if err := os.WriteFile(keyFile, []byte(keyContent), 0600); err != nil {
		return err
	}
	
	return nil
}

func (s *BrandService) LoadTLSCertificates() (certContent, keyContent string, err error) {
	var settings models.BrandSettings
	
	result := db.DB.Session(&gorm.Session{Logger: gormlogger.Discard}).First(&settings)
	if result.Error != nil {
		return "", "", result.Error
	}
	
	return settings.TLSCertContent, settings.TLSKeyContent, nil
}
