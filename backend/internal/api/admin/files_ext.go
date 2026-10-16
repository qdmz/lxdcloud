package admin

// 后台容器文件扩展接口（打包/解压/权限/属主/写入）：
// 用户侧 /api/user/containers/:name/files/* 经 UserNodeProxy 中间件按容器 NodeID 转发到
// 节点后台的同名 /api/admin/... 接口，因此节点容器的新文件管理功能依赖这组 admin 路由存在。
// 逻辑与 user/files_ext.go 保持一致，仅把属主校验换成存在性校验（后台已鉴权）。

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"lxdapi/internal/service"
	"lxdapi/pkg/logger"
	"lxdapi/pkg/response"
)

func adminShQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\\''`) + "'"
}

func adminExecIn(c *gin.Context, name string, args ...string) (string, error) {
	svc := service.NewContainerService()
	return svc.GetLXCClient().ExecInContainer(c.Request.Context(), name, args)
}

// ArchiveFile 打包下载目录或文件（tar.gz）
// @Router /api/admin/containers/:name/files/archive [get]
func ArchiveFile(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		response.Error(c, 400, "缺少容器名称")
		return
	}
	if err := checkContainerExists(name); err != nil {
		response.Error(c, 404, err.Error())
		return
	}
	p, err := cleanContainerPath(c.Query("path"))
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	base := path.Base(p)
	if base == "" || base == "/" || base == "." {
		response.Error(c, 400, "无效的路径")
		return
	}
	parent := path.Dir(p)
	// 打包前先估算体积——ExecInContainer 会把全部输出缓冲进内存，大归档可能把 lxdapi 进程打爆
	const maxArchiveBytes = 500 * 1024 * 1024
	if szOut, szErr := adminExecIn(c, name, "sh", "-c", fmt.Sprintf("du -sb %s | cut -f1", adminShQuote(p))); szErr == nil {
		if sz, convErr := strconv.ParseInt(strings.TrimSpace(szOut), 10, 64); convErr == nil && sz > maxArchiveBytes {
			response.Error(c, 400, fmt.Sprintf("打包目标过大（约 %.1f MB），超过 500MB 上限，请分批打包", float64(sz)/1048576))
			return
		}
	}
	// busybox 的 base64（Alpine 容器）不支持 -w0；去掉参数，解码前由 Go 去掉所有空白字符兼容换行输出
	cmd := fmt.Sprintf("tar -czf - -C %s %s | base64", adminShQuote(parent), adminShQuote(base))
	out, err := adminExecIn(c, name, "sh", "-c", cmd)
	if err != nil {
		logger.Error("打包失败: %v", err)
		response.Error(c, 500, "打包失败: "+err.Error())
		return
	}
	cleaned := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, out)
	raw, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		response.Error(c, 500, "打包数据解码失败")
		return
	}
	fn := base + ".tar.gz"
	c.Header("Content-Disposition", "attachment; filename=\""+fn+"\"; filename*=UTF-8''"+url.PathEscape(fn))
	c.Data(200, "application/gzip", raw)
}

// ExtractFile 解压容器内压缩包
// @Router /api/admin/containers/:name/files/extract [post]
func ExtractFile(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		response.Error(c, 400, "缺少容器名称")
		return
	}
	if err := checkContainerExists(name); err != nil {
		response.Error(c, 404, err.Error())
		return
	}
	var req struct {
		Path string `json:"path" binding:"required"`
		Dest string `json:"dest"`
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
	destDir := path.Dir(p)
	if req.Dest != "" {
		d, err := cleanContainerPath(req.Dest)
		if err != nil {
			response.Error(c, 400, err.Error())
			return
		}
		destDir = d
	}
	lower := strings.ToLower(p)
	var cmd string
	if strings.HasSuffix(lower, ".zip") {
		cmd = fmt.Sprintf("mkdir -p %s && (unzip -o %s -d %s || python3 -m zipfile -e %s %s) 2>&1 || echo __NEED_UNZIP__", adminShQuote(destDir), adminShQuote(p), adminShQuote(destDir), adminShQuote(p), adminShQuote(destDir))
	} else {
		cmd = fmt.Sprintf("mkdir -p %s && tar -xf %s -C %s 2>&1", adminShQuote(destDir), adminShQuote(p), adminShQuote(destDir))
	}
	out, err := adminExecIn(c, name, "sh", "-c", cmd)
	txt := strings.TrimSpace(out)
	if err != nil {
		logger.Error("解压失败: %v", err)
		response.Error(c, 500, "解压失败: "+err.Error()+" "+txt)
		return
	}
	// 只依赖 __NEED_UNZIP__ 标记：unzip 正常输出里也可能出现 "not found"（如 filename not matched），不能做二次判断
	if strings.Contains(out, "__NEED_UNZIP__") {
		response.Error(c, 400, "容器内缺少 unzip，无法解压 zip（请先安装 unzip 或改用 tar.gz）")
		return
	}
	response.Success(c, gin.H{"path": p, "dest": destDir, "log": txt})
}

var adminFileModeRe = regexp.MustCompile(`^[0-7]{3,4}$`)
var adminOwnerNameRe = regexp.MustCompile(`^[A-Za-z0-9_.\-]{1,32}$`)

// ChmodFile 修改文件/目录权限
// @Router /api/admin/containers/:name/files/chmod [post]
func ChmodFile(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		response.Error(c, 400, "缺少容器名称")
		return
	}
	if err := checkContainerExists(name); err != nil {
		response.Error(c, 404, err.Error())
		return
	}
	var req struct {
		Path string `json:"path" binding:"required"`
		Mode string `json:"mode" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	if !adminFileModeRe.MatchString(req.Mode) {
		response.Error(c, 400, "权限格式应为 3-4 位八进制，如 755 / 0644")
		return
	}
	p, err := cleanContainerPath(req.Path)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	if _, err := adminExecIn(c, name, "chmod", req.Mode, p); err != nil {
		logger.Error("修改权限失败: %v", err)
		response.Error(c, 500, "修改权限失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{"path": p, "mode": req.Mode})
}

// ChownFile 修改文件/目录属主属组
// @Router /api/admin/containers/:name/files/chown [post]
func ChownFile(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		response.Error(c, 400, "缺少容器名称")
		return
	}
	if err := checkContainerExists(name); err != nil {
		response.Error(c, 404, err.Error())
		return
	}
	var req struct {
		Path  string `json:"path" binding:"required"`
		Owner string `json:"owner"`
		Group string `json:"group"`
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
	if req.Owner == "" && req.Group == "" {
		response.Error(c, 400, "请提供属主或属组")
		return
	}
	if req.Owner != "" && !adminOwnerNameRe.MatchString(req.Owner) {
		response.Error(c, 400, "属主名非法")
		return
	}
	if req.Group != "" && !adminOwnerNameRe.MatchString(req.Group) {
		response.Error(c, 400, "属组名非法")
		return
	}
	spec := req.Owner
	if req.Group != "" {
		spec = req.Owner + ":" + req.Group
	}
	if _, err := adminExecIn(c, name, "chown", spec, p); err != nil {
		logger.Error("修改属主失败: %v", err)
		response.Error(c, 500, "修改属主失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{"path": p, "owner": req.Owner, "group": req.Group})
}

// WriteFile 新建或覆盖写入文件内容（用于新建空文件与在线编辑保存）
// @Router /api/admin/containers/:name/files/write [post]
func WriteFile(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		response.Error(c, 400, "缺少容器名称")
		return
	}
	if err := checkContainerExists(name); err != nil {
		response.Error(c, 404, err.Error())
		return
	}
	var req struct {
		Path     string `json:"path" binding:"required"`
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
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
		response.Error(c, 400, "无效的文件路径")
		return
	}
	if _, derr := adminExecIn(c, name, "test", "-d", p); derr == nil {
		response.Error(c, 400, "目标是一个目录，无法写入文件")
		return
	}
	var data []byte
	if strings.EqualFold(req.Encoding, "base64") {
		d, decErr := base64.StdEncoding.DecodeString(strings.TrimSpace(req.Content))
		if decErr != nil {
			response.Error(c, 400, "base64 内容解码失败")
			return
		}
		data = d
	} else {
		data = []byte(req.Content)
	}
	if len(data) > 5*1024*1024 {
		response.Error(c, 400, "文件内容过大（上限 5MB），请改用上传功能")
		return
	}
	client := lxdFileHTTPClient()
	httpReq, _ := http.NewRequest("POST", lxdFileURL(name, p), bytes.NewReader(data))
	httpReq.Header.Set("Content-Type", "application/octet-stream")
	httpReq.ContentLength = int64(len(data))
	resp, err := client.Do(httpReq)
	if err != nil {
		logger.Error("写入文件失败: %v", err)
		response.Error(c, 500, "写入文件失败")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		response.Error(c, 500, "写入文件失败: LXD 返回 "+resp.Status)
		return
	}
	response.Success(c, gin.H{"path": p, "size": len(data)})
}
