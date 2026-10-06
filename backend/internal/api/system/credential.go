package system

import (
	"github.com/gin-gonic/gin"
	"lxdapi/internal/service"
	"lxdapi/pkg/logger"
	"lxdapi/pkg/response"
)

// GetContainerCredential 获取容器访问码
// @Summary 获取容器访问码
// @Description 获取容器的访问码，如果不存在则自动创建
// @Tags System API - 凭证管理
// @Accept json
// @Produce json
// @Param name path string true "容器名称"
// @Success 200 {object} response.Response "获取成功"
// @Failure 400 {object} response.Response "缺少容器名称"
// @Failure 500 {object} response.Response "获取失败"
// @Security ApiKeyAuth
// @Router /api/system/containers/{name}/credential [get]
func GetContainerCredential(c *gin.Context) {
	containerName := c.Param("name")
	if containerName == "" {
		response.Error(c, 400, "缺少容器名称")
		return
	}

	credential, err := service.GetContainerCredential(containerName)
	if err != nil {
		credential, err = service.CreateContainerCredential(containerName)
		if err != nil {
			logger.Error("创建容器访问码失败: %v", err)
			response.Error(c, 500, err.Error())
			return
		}
		logger.OK("自动创建容器访问码: %s", containerName)
	}

	response.Success(c, gin.H{
		"container_name": credential.ContainerName,
		"access_code":    credential.Hash,
		"jump_url":       "/container/dashboard?hash=" + credential.Hash,
		"created_at":     credential.CreatedAt,
	})
}

// RegenerateContainerCredential 重新生成容器访问码
// @Summary 重新生成容器访问码
// @Description 重新生成容器的访问码
// @Tags System API - 凭证管理
// @Accept json
// @Produce json
// @Param name path string true "容器名称"
// @Success 200 {object} response.Response "生成成功"
// @Failure 400 {object} response.Response "缺少容器名称"
// @Failure 500 {object} response.Response "生成失败"
// @Security ApiKeyAuth
// @Router /api/system/containers/{name}/credential/regenerate [post]
func RegenerateContainerCredential(c *gin.Context) {
	containerName := c.Param("name")
	if containerName == "" {
		response.Error(c, 400, "缺少容器名称")
		return
	}

	credential, err := service.RegenerateContainerHash(containerName)
	if err != nil {
		logger.Error("重新生成容器访问码失败: %v", err)
		response.Error(c, 500, err.Error())
		return
	}

	logger.OK("重新生成容器访问码成功: %s", containerName)
	response.Success(c, gin.H{
		"container_name": credential.ContainerName,
		"access_code":    credential.Hash,
		"jump_url":       "/container/dashboard?hash=" + credential.Hash,
		"updated_at":     credential.UpdatedAt,
	})
}
