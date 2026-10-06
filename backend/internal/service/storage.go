package service

import (
	"context"
	"lxdapi/internal/db"
	"lxdapi/internal/lxc"
	"lxdapi/models"
	"lxdapi/pkg/logger"
)

type StorageService struct {
	lxcClient *lxc.Client
}

func NewStorageService() *StorageService {
	return &StorageService{lxcClient: lxc.NewClient()}
}

func (s *StorageService) List() ([]models.StoragePool, error) {
	var pools []models.StoragePool
	err := db.DB.Order("priority ASC, name ASC").Find(&pools).Error
	return pools, err
}

func (s *StorageService) SyncFromLXD(ctx context.Context) (int, int, int, error) {
	logger.Info("开始从LXD同步存储池")

	pools, err := s.lxcClient.ListStoragePools(ctx)
	if err != nil {
		return 0, 0, 0, err
	}

	added, updated := 0, 0
	lxdPools := make(map[string]bool)

	for _, p := range pools {
		lxdPools[p.Name] = true

		var totalSpace, usedSpace int64
		if res, err := s.lxcClient.GetStoragePoolResources(ctx, p.Name); err == nil {
			totalSpace = res.Space.Total
			usedSpace = res.Space.Used
		}

		var existing models.StoragePool
		err := db.DB.Where("name = ?", p.Name).First(&existing).Error

		pool := models.StoragePool{
			Name:        p.Name,
			Driver:      p.Driver,
			Description: p.Description,
			Status:      p.Status,
			UsedBy:      len(p.UsedBy),
			TotalSpace:  totalSpace,
			UsedSpace:   usedSpace,
		}

		if err != nil {
			pool.Priority = 2
			if p.Name == "default" {
				pool.Priority = 1
			}
			db.DB.Create(&pool)
			added++
			logger.Info("添加存储池: %s (%s)", p.Name, p.Driver)
		} else {
			pool.Priority = existing.Priority
			db.DB.Model(&existing).Updates(pool)
			updated++
		}
	}

	var dbPools []models.StoragePool
	db.DB.Find(&dbPools)
	deleted := 0
	for _, dbPool := range dbPools {
		if !lxdPools[dbPool.Name] {
			db.DB.Unscoped().Delete(&dbPool)
			deleted++
			logger.Info("删除过期存储池: %s", dbPool.Name)
		}
	}

	logger.OK("同步完成: 新增 %d, 更新 %d, 删除 %d", added, updated, deleted)
	return added, updated, deleted, nil
}

func (s *StorageService) SetPriority(name string, priority int) error {
	return db.DB.Model(&models.StoragePool{}).Where("name = ?", name).Update("priority", priority).Error
}

func (s *StorageService) GetDefault() string {
	ctx := context.Background()
	var pools []models.StoragePool
	db.DB.Where("priority > 0").Order("priority ASC").Find(&pools)

	for _, pool := range pools {
		res, err := s.lxcClient.GetStoragePoolResources(ctx, pool.Name)
		if err != nil {
			continue
		}
		if res.Space.Total > 0 && float64(res.Space.Used)/float64(res.Space.Total) < 0.95 {
			return pool.Name
		}
	}
	return "default"
}
