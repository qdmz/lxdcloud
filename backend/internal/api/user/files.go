package user

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"lxdapi/internal/core"
	"lxdapi/internal/service"
	"lxdapi/pkg/logger"
	"lxdapi/pkg/response"
)

// lxdFileHTTPClient 返回通过 unix socket 连接 LXD REST 的 HTTP 客户端
func lxdFileHTTPClient() *http.Client {
	socket := core.GlobalConfig.LXC.Socket
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socket)
			},
		},
	}
}

func cleanContainerPath(p string) (string, error) {
	if p == "" {
		p = "/"
	}
	if !strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("路径必须是绝对路径")
	}
	if strings.Contains(p, "..") {
		return "", fmt.Errorf("路径不允许包含 ..")
	}
	return path.Clean(p), nil
}

// checkContainerOwner 校验当前用户是否有权访问指定容器
func checkContainerOwner(c *gin.Context, name string) error {
	username, _ := c.Get("username")
	container, err := containerService.Get(name)
	if err != nil {
		return fmt.Errorf("容器不存在")
	}
	if container.UserID != username.(string) {
		return fmt.Errorf("无权访问该容器")
	}
	return nil
}

// ListFiles 列出容器目录
// @Summary 列出容器目录
// @Description 列出容器内指定目录的文件列表
// @Tags User API - 容器文件
// @Produce json
// @Param name path string true "容器名称"
// @Param path query string false "目录路径，默认 /"
// @Success 200 {object} response.Response "文件列表"
// @Failure 403 {object} response.Response "无权访问"
// @Security UserSession
// @Router /api/user/containers/:name/files [get]
func ListFiles(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		response.Error(c, 400, "缺少容器名称")
		return
	}
	if err := checkContainerOwner(c, name); err != nil {
		response.Error(c, 403, err.Error())
		return
	}

	p, err := cleanContainerPath(c.Query("path"))
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	svc := service.NewContainerService()
	out, err := svc.GetLXCClient().ExecInContainer(c.Request.Context(), name, []string{"ls", "-la", "--full-time", p})
	if err != nil {
		logger.Error("列目录失败: %v", err)
		response.Error(c, 500, "列目录失败: "+err.Error())
		return
	}

	entries := parseLsOutput(out, p)
	response.Success(c, gin.H{
		"path":    p,
		"entries": entries,
	})
}

// parseLsOutput 解析 busybox ls -la --full-time 输出
func parseLsOutput(out, basePath string) []gin.H {
	var items []gin.H
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "total ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}
		mode := fields[0]
		links := fields[1]
		owner := fields[2]
		group := fields[3]
		size := fields[4]
		date := fields[5]
		time := fields[6]
		// tz := fields[7]
		rest := strings.Join(fields[8:], " ")
		// 符号链接显示 "name -> target"
		displayName := rest
		linkTarget := ""
		if idx := strings.Index(rest, " -> "); idx >= 0 {
			displayName = rest[:idx]
			linkTarget = rest[idx+4:]
		}
		if displayName == "." || displayName == ".." {
			continue
		}
		isDir := strings.HasPrefix(mode, "d")
		items = append(items, gin.H{
			"name":      displayName,
			"path":      path.Join(basePath, displayName),
			"is_dir":    isDir,
			"mode":      mode,
			"links":     links,
			"owner":     owner,
			"group":     group,
			"size":      parseIntSafe(size),
			"mtime":     date + " " + time,
			"link":      linkTarget,
			"is_symlink": strings.HasPrefix(mode, "l"),
		})
	}
	return items
}

func parseIntSafe(s string) int64 {
	var n int64
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			break
		}
		n = n*10 + int64(ch-'0')
	}
	return n
}

// DownloadFile 下载容器内文件
// @Summary 下载容器内文件
// @Description 下载容器内指定路径的文件内容
// @Tags User API - 容器文件
// @Produce application/octet-stream
// @Param name path string true "容器名称"
// @Param path query string true "文件路径"
// @Success 200 {file} binary "文件内容"
// @Failure 403 {object} response.Response "无权访问"
// @Security UserSession
// @Router /api/user/containers/:name/files/download [get]
func DownloadFile(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		response.Error(c, 400, "缺少容器名称")
		return
	}
	if err := checkContainerOwner(c, name); err != nil {
		response.Error(c, 403, err.Error())
		return
	}

	p, err := cleanContainerPath(c.Query("path"))
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	client := lxdFileHTTPClient()
	url := fmt.Sprintf("http://lxd/1.0/instances/%s/files?path=%s", name, p)
	req, _ := http.NewRequest("GET", url, nil)
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("下载文件失败: %v", err)
		response.Error(c, 500, "下载文件失败")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		response.Error(c, 500, "下载文件失败: LXD 返回 "+resp.Status)
		return
	}

	fileName := path.Base(p)
	c.Header("Content-Disposition", "attachment; filename=\""+fileName+"\"")
	c.DataFromReader(200, resp.ContentLength, "application/octet-stream", resp.Body, nil)
}

// UploadFile 上传文件到容器
// @Summary 上传文件到容器
// @Description 将文件上传到容器内指定路径（multipart/form-data, 字段名 file）
// @Tags User API - 容器文件
// @Accept multipart/form-data
// @Param name path string true "容器名称"
// @Param path formData string true "目标文件路径"
// @Param file formData file true "文件内容"
// @Success 200 {object} response.Response "上传成功"
// @Failure 403 {object} response.Response "无权访问"
// @Security UserSession
// @Router /api/user/containers/:name/files/upload [post]
func UploadFile(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		response.Error(c, 400, "缺少容器名称")
		return
	}
	if err := checkContainerOwner(c, name); err != nil {
		response.Error(c, 403, err.Error())
		return
	}

	p, err := cleanContainerPath(c.PostForm("path"))
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Error(c, 400, "缺少上传文件")
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		response.Error(c, 500, "读取上传文件失败")
		return
	}
	defer f.Close()

	client := lxdFileHTTPClient()
	url := fmt.Sprintf("http://lxd/1.0/instances/%s/files?path=%s", name, p)
	req, _ := http.NewRequest("POST", url, f)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = fileHeader.Size

	resp, err := client.Do(req)
	if err != nil {
		logger.Error("上传文件失败: %v", err)
		response.Error(c, 500, "上传文件失败")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		response.Error(c, 500, "上传文件失败: LXD 返回 "+resp.Status)
		return
	}

	response.Success(c, gin.H{"path": p})
}

// MakeDir 在容器内创建目录
// @Summary 创建目录
// @Description 在容器内指定路径创建目录
// @Tags User API - 容器文件
// @Accept json
// @Produce json
// @Param name path string true "容器名称"
// @Param request body object{path=string} true "目录路径"
// @Success 200 {object} response.Response "创建成功"
// @Failure 403 {object} response.Response "无权访问"
// @Security UserSession
// @Router /api/user/containers/:name/files/mkdir [post]
func MakeDir(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		response.Error(c, 400, "缺少容器名称")
		return
	}
	if err := checkContainerOwner(c, name); err != nil {
		response.Error(c, 403, err.Error())
		return
	}

	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}

	p, err := cleanContainerPath(req.Path)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	client := lxdFileHTTPClient()
	url := fmt.Sprintf("http://lxd/1.0/instances/%s/files?path=%s", name, p)
	req2, _ := http.NewRequest("POST", url, nil)
	req2.Header.Set("X-LXD-type", "directory")

	resp, err := client.Do(req2)
	if err != nil {
		logger.Error("创建目录失败: %v", err)
		response.Error(c, 500, "创建目录失败")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		response.Error(c, 500, "创建目录失败: LXD 返回 "+resp.Status)
		return
	}

	response.Success(c, gin.H{"path": p})
}

// DeleteFile 删除容器内文件或目录
// @Summary 删除文件/目录
// @Description 删除容器内指定路径的文件或目录（目录递归删除）
// @Tags User API - 容器文件
// @Accept json
// @Produce json
// @Param name path string true "容器名称"
// @Param request body object{path=string} true "目标路径"
// @Success 200 {object} response.Response "删除成功"
// @Failure 403 {object} response.Response "无权访问"
// @Security UserSession
// @Router /api/user/containers/:name/files/delete [post]
func DeleteFile(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		response.Error(c, 400, "缺少容器名称")
		return
	}
	if err := checkContainerOwner(c, name); err != nil {
		response.Error(c, 403, err.Error())
		return
	}

	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}

	p, err := cleanContainerPath(req.Path)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	if p == "/" {
		response.Error(c, 400, "禁止删除根目录")
		return
	}

	svc := service.NewContainerService()
	_, err = svc.GetLXCClient().ExecInContainer(c.Request.Context(), name, []string{"rm", "-rf", "--", p})
	if err != nil {
		logger.Error("删除失败: %v", err)
		response.Error(c, 500, "删除失败: "+err.Error())
		return
	}

	response.Success(c, gin.H{"path": p})
}

// RenameFile 重命名/移动容器内文件
// @Summary 重命名/移动文件
// @Description 重命名或移动容器内文件/目录
// @Tags User API - 容器文件
// @Accept json
// @Produce json
// @Param name path string true "容器名称"
// @Param request body object{src=string,dst=string} true "源路径与目标路径"
// @Success 200 {object} response.Response "操作成功"
// @Failure 403 {object} response.Response "无权访问"
// @Security UserSession
// @Router /api/user/containers/:name/files/rename [post]
func RenameFile(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		response.Error(c, 400, "缺少容器名称")
		return
	}
	if err := checkContainerOwner(c, name); err != nil {
		response.Error(c, 403, err.Error())
		return
	}

	var req struct {
		Src string `json:"src" binding:"required"`
		Dst string `json:"dst" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}

	src, err := cleanContainerPath(req.Src)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	dst, err := cleanContainerPath(req.Dst)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	svc := service.NewContainerService()
	_, err = svc.GetLXCClient().ExecInContainer(c.Request.Context(), name, []string{"mv", "--", src, dst})
	if err != nil {
		logger.Error("重命名失败: %v", err)
		response.Error(c, 500, "重命名失败: "+err.Error())
		return
	}

	response.Success(c, gin.H{"src": src, "dst": dst})
}
