package admin

// v1.1.22 镜像管理 / 远程镜像下载 / 节点包缓存（仅管理端）。
// 带 node_id 的请求由 AdminNodeProxy 转发到节点执行（节点本地资源只存在节点上）。

import (
	"lxdapi/internal/db"
	"lxdapi/internal/lxc"
	"lxdapi/internal/service"
	"lxdapi/models"
	"lxdapi/pkg/response"
	"strings"

	"github.com/gin-gonic/gin"
)

func requireLXD(c *gin.Context) bool {
	if !service.LocalLXDAvailable() {
		response.Error(c, 503, "本机未连接 LXD（主控请在页面右上角切换到节点）")
		return false
	}
	return true
}

// ListImages 节点本地镜像
// @Summary 节点本地镜像列表
// @Tags Admin API - 镜像管理
// @Produce json
// @Param node_id query int false "节点 ID（主控转发）"
// @Success 200 {object} response.Response
// @Security SessionAuth
// @Router /api/admin/images [get]
func ListImages(c *gin.Context) {
	if !requireLXD(c) {
		return
	}
	list, err := service.ListLocalImages(c.Request.Context())
	if err != nil {
		response.Error(c, 500, "获取镜像列表失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{"images": list, "count": len(list), "host_arch": service.HostArch()})
}

// DeleteImage 删除节点本地镜像
// @Summary 删除镜像
// @Tags Admin API - 镜像管理
// @Param fingerprint path string true "镜像指纹"
// @Success 200 {object} response.Response
// @Security SessionAuth
// @Router /api/admin/images/{fingerprint} [delete]
func DeleteImage(c *gin.Context) {
	fp := strings.ToLower(c.Param("fingerprint"))
	if !service.ValidFingerprint(fp) {
		response.Error(c, 400, "指纹无效")
		return
	}
	if !requireLXD(c) {
		return
	}
	if err := service.DeleteLocalImage(c.Request.Context(), fp); err != nil {
		response.Error(c, 500, err.Error())
		return
	}
	response.Success(c, gin.H{"message": "镜像已删除"})
}

// UpdateImageAliases 设置镜像别名（整体替换）
// @Summary 编辑镜像别名
// @Tags Admin API - 镜像管理
// @Accept json
// @Param fingerprint path string true "镜像指纹"
// @Param request body object{aliases=[]string} true "别名列表"
// @Success 200 {object} response.Response
// @Security SessionAuth
// @Router /api/admin/images/{fingerprint}/aliases [put]
func UpdateImageAliases(c *gin.Context) {
	fp := strings.ToLower(c.Param("fingerprint"))
	if !service.ValidFingerprint(fp) {
		response.Error(c, 400, "指纹无效")
		return
	}
	var req struct {
		Aliases []string `json:"aliases"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误: "+err.Error())
		return
	}
	for _, a := range req.Aliases {
		if a = strings.TrimSpace(a); a != "" && !service.ValidImageAlias(a) {
			response.Error(c, 400, "别名 "+a+" 无效（字母数字开头，可含 . _ / -，最长 128）")
			return
		}
	}
	if !requireLXD(c) {
		return
	}
	list, err := service.SetImageAliases(c.Request.Context(), fp, req.Aliases)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"aliases": list})
}

// ListImageSources 镜像源（内置 + 自定义，存于主控）
// @Summary 镜像源列表
// @Tags Admin API - 镜像管理
// @Success 200 {object} response.Response
// @Security SessionAuth
// @Router /api/admin/image-sources [get]
func ListImageSources(c *gin.Context) {
	response.Success(c, gin.H{"sources": service.ListImageSources()})
}

// CreateImageSource 新增自定义镜像源
// @Summary 新增镜像源
// @Tags Admin API - 镜像管理
// @Accept json
// @Param request body object{name=string,url=string,protocol=string} true "镜像源"
// @Success 200 {object} response.Response
// @Security SessionAuth
// @Router /api/admin/image-sources [post]
func CreateImageSource(c *gin.Context) {
	var req struct {
		Name     string `json:"name"`
		URL      string `json:"url"`
		Protocol string `json:"protocol"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误: "+err.Error())
		return
	}
	if req.Protocol == "" {
		req.Protocol = "simplestreams"
	}
	s, err := service.AddImageSource(req.Name, req.URL, req.Protocol)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, s)
}

// DeleteImageSource 删除自定义镜像源
// @Summary 删除镜像源
// @Tags Admin API - 镜像管理
// @Param id path int true "ID"
// @Success 200 {object} response.Response
// @Security SessionAuth
// @Router /api/admin/image-sources/{id} [delete]
func DeleteImageSource(c *gin.Context) {
	id := c.Param("id")
	for _, ch := range id {
		if ch < '0' || ch > '9' {
			response.Error(c, 400, "内置镜像源不能删除")
			return
		}
	}
	if err := db.DB.Delete(&models.ImageSource{}, id).Error; err != nil {
		response.Error(c, 500, err.Error())
		return
	}
	response.Success(c, gin.H{"message": "已删除"})
}

// RemoteImages 浏览远程镜像目录
// @Summary 远程镜像目录
// @Tags Admin API - 镜像管理
// @Param server query string true "镜像源地址"
// @Param protocol query string false "simplestreams/lxd"
// @Param refresh query int false "1=忽略缓存"
// @Success 200 {object} response.Response
// @Security SessionAuth
// @Router /api/admin/images/remote [get]
func RemoteImages(c *gin.Context) {
	server := c.Query("server")
	protocol := c.DefaultQuery("protocol", "simplestreams")
	if _, err := service.ValidateRemoteServer(server, protocol); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	items, err := service.FetchRemoteCatalog(c.Request.Context(), server, protocol, c.Query("refresh") == "1")
	if err != nil {
		response.Error(c, 502, "读取远程镜像目录失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{"images": items, "count": len(items), "host_arch": service.HostArch()})
}

// DownloadImage 从远程下载镜像到节点（异步作业，加入任务队列）
// @Summary 下载远程镜像
// @Tags Admin API - 镜像管理
// @Accept json
// @Param request body service.ImageDownloadRequest true "下载参数"
// @Success 200 {object} response.Response
// @Security SessionAuth
// @Router /api/admin/images/download [post]
func DownloadImage(c *gin.Context) {
	var req service.ImageDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误: "+err.Error())
		return
	}
	if err := req.Validate(); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	if !requireLXD(c) {
		return
	}
	job, err := service.StartImageDownload(c.Request.Context(), req)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, job)
}

// ImportImageURL 从 URL 导入 tar 镜像（异步作业）
// @Summary 从 URL 导入镜像
// @Tags Admin API - 镜像管理
// @Accept json
// @Param request body service.ImageImportURLRequest true "导入参数"
// @Success 200 {object} response.Response
// @Security SessionAuth
// @Router /api/admin/images/import-url [post]
func ImportImageURL(c *gin.Context) {
	var req service.ImageImportURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误: "+err.Error())
		return
	}
	if err := req.Validate(); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	if !requireLXD(c) {
		return
	}
	job, err := service.StartImageImportURL(c.Request.Context(), req)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, job)
}

// ImageJobs 镜像下载/导入作业进度
// @Summary 镜像作业进度
// @Tags Admin API - 镜像管理
// @Success 200 {object} response.Response
// @Security SessionAuth
// @Router /api/admin/images/jobs [get]
func ImageJobs(c *gin.Context) {
	jobs := service.ListJobs("image_download", "image_import")
	if jobs == nil {
		jobs = []service.BgJob{}
	}
	response.Success(c, gin.H{"jobs": jobs})
}

// GetPkgCache 节点包缓存状态
// @Summary 包缓存状态
// @Tags Admin API - 包缓存
// @Success 200 {object} response.Response
// @Security SessionAuth
// @Router /api/admin/pkgcache [get]
func GetPkgCache(c *gin.Context) {
	response.Success(c, service.GetPkgCacheStatus(c.Request.Context()))
}

// SavePkgCacheSettings 保存预装包列表等设置
// @Summary 包缓存设置
// @Tags Admin API - 包缓存
// @Accept json
// @Param request body lxc.PkgCacheSettings true "设置"
// @Success 200 {object} response.Response
// @Security SessionAuth
// @Router /api/admin/pkgcache/settings [put]
func SavePkgCacheSettings(c *gin.Context) {
	var req lxc.PkgCacheSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误: "+err.Error())
		return
	}
	for _, p := range req.Packages {
		if p = strings.ToLower(strings.TrimSpace(p)); p != "" && !lxc.ValidPkgName(p) {
			response.Error(c, 400, "包名 "+p+" 无效（小写字母数字开头，可含 . + _ -）")
			return
		}
	}
	if req.OnlineTimeout < 0 || req.OnlineTimeout > 1800 {
		response.Error(c, 400, "在线安装超时需在 0~1800 秒之间")
		return
	}
	if err := lxc.SavePkgCacheSettings(req); err != nil {
		response.Error(c, 500, err.Error())
		return
	}
	response.Success(c, lxc.LoadPkgCacheSettings())
}

// SyncPkgCache 触发同步（下载）
// @Summary 同步包缓存
// @Tags Admin API - 包缓存
// @Accept json
// @Param request body service.PkgCacheSyncRequest true "同步参数"
// @Success 200 {object} response.Response
// @Security SessionAuth
// @Router /api/admin/pkgcache/sync [post]
func SyncPkgCache(c *gin.Context) {
	var req service.PkgCacheSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误: "+err.Error())
		return
	}
	if len(req.Images) == 0 && !req.All {
		response.Error(c, 400, "请选择镜像")
		return
	}
	job, err := service.StartPkgCacheSync(c.Request.Context(), req)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, job)
}

// DeletePkgCache 删除某个缓存目录
// @Summary 删除包缓存
// @Tags Admin API - 包缓存
// @Param key query string true "发行版/版本/架构"
// @Success 200 {object} response.Response
// @Security SessionAuth
// @Router /api/admin/pkgcache/entry [delete]
func DeletePkgCache(c *gin.Context) {
	parts := strings.Split(c.Query("key"), "/")
	if len(parts) != 3 {
		response.Error(c, 400, "key 格式应为 发行版/版本/架构")
		return
	}
	if err := lxc.DeletePkgCacheEntry(parts[0], parts[1], parts[2]); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"message": "已删除"})
}
