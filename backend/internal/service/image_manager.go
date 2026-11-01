package service

// 镜像管理与远程镜像下载（v1.1.22，仅管理端；节点本地资源，主控按 node_id 转发到节点执行）
//
// 参考 xkatld/lxdapi-web-server 的「镜像模板管理」（本地 lxc image list/delete + 脚本 lxc image import 预构建 tar），
// 这里补充：按节点列出/删除/编辑别名、浏览 simplestreams/LXD 远程镜像目录并下载到节点、从 URL 导入 tar 镜像。
// 下载走 LXD REST（lxc query POST /1.0/images，source.type=image，等价于 Go client 的 CopyImage），
// 异步操作轮询进度；仅提供 incus.tar.xz 的源（linuxcontainers.org 及其国内镜像）LXD 无法直接拉取，
// 改由节点下载 meta + rootfs 后 lxc image import。

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"lxdapi/internal/core"
	"lxdapi/internal/db"
	"lxdapi/internal/lxc"
	"lxdapi/models"
	"lxdapi/pkg/logger"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ImageSourceInfo 镜像源
type ImageSourceInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Protocol string `json:"protocol"`
	Builtin  bool   `json:"builtin"`
	Note     string `json:"note,omitempty"`
}

// BuiltinImageSources 内置镜像源（2026-11 实测可访问）
var BuiltinImageSources = []ImageSourceInfo{
	{ID: "lxd-images", Name: "LXD 官方镜像（images:）", URL: "https://images.lxd.canonical.com", Protocol: "simplestreams", Builtin: true, Note: "LXD 默认 images: 远程，含 Debian/Alpine/Rocky 等，不含 Ubuntu"},
	{ID: "ubuntu", Name: "Ubuntu 官方云镜像（ubuntu:）", URL: "https://cloud-images.ubuntu.com/releases", Protocol: "simplestreams", Builtin: true},
	{ID: "ubuntu-minimal", Name: "Ubuntu Minimal 云镜像", URL: "https://cloud-images.ubuntu.com/minimal/releases", Protocol: "simplestreams", Builtin: true},
	{ID: "tuna", Name: "清华 TUNA lxc-images", URL: "https://mirrors.tuna.tsinghua.edu.cn/lxc-images", Protocol: "simplestreams", Builtin: true, Note: "linuxcontainers.org 镜像（Incus 格式），由节点下载后导入"},
	{ID: "bfsu", Name: "北外 BFSU lxc-images", URL: "https://mirrors.bfsu.edu.cn/lxc-images", Protocol: "simplestreams", Builtin: true, Note: "同上，Incus 格式由节点下载后导入"},
	{ID: "nju-ubuntu", Name: "南大 NJU Ubuntu 云镜像", URL: "https://mirrors.nju.edu.cn/ubuntu-cloud-images/releases", Protocol: "simplestreams", Builtin: true},
	{ID: "linuxcontainers", Name: "linuxcontainers.org", URL: "https://images.linuxcontainers.org", Protocol: "simplestreams", Builtin: true, Note: "Incus 格式，由节点下载后导入"},
}

// ListImageSources 内置 + 自定义镜像源
func ListImageSources() []ImageSourceInfo {
	out := append([]ImageSourceInfo{}, BuiltinImageSources...)
	if db.DB != nil {
		var list []models.ImageSource
		db.DB.Order("id").Find(&list)
		for _, s := range list {
			out = append(out, ImageSourceInfo{ID: strconv.FormatUint(uint64(s.ID), 10), Name: s.Name, URL: s.URL, Protocol: s.Protocol})
		}
	}
	return out
}

// ValidateRemoteServer 校验远程地址与协议，返回规范化地址
func ValidateRemoteServer(server, protocol string) (string, error) {
	server = strings.TrimSpace(server)
	u, err := url.Parse(server)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil {
		return "", fmt.Errorf("镜像源地址需为 http(s):// 开头的有效地址")
	}
	if protocol != "simplestreams" && protocol != "lxd" {
		return "", fmt.Errorf("协议只能是 simplestreams 或 lxd")
	}
	if protocol == "lxd" && u.Scheme != "https" {
		return "", fmt.Errorf("LXD 远程需使用 https")
	}
	u.RawQuery, u.Fragment = "", ""
	return strings.TrimRight(u.String(), "/"), nil
}

// AddImageSource 新增自定义镜像源
func AddImageSource(name, server, protocol string) (*models.ImageSource, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 50 {
		return nil, fmt.Errorf("名称不能为空且不超过 50 字")
	}
	u, err := ValidateRemoteServer(server, protocol)
	if err != nil {
		return nil, err
	}
	s := &models.ImageSource{Name: name, URL: u, Protocol: protocol}
	if err := db.DB.Create(s).Error; err != nil {
		return nil, err
	}
	return s, nil
}

// ---------------- 本地镜像 ----------------

// LocalImage 节点本地镜像
type LocalImage struct {
	Fingerprint  string   `json:"fingerprint"`
	Aliases      []string `json:"aliases"`
	OS           string   `json:"os"`
	Release      string   `json:"release"`
	Variant      string   `json:"variant"`
	Description  string   `json:"description"`
	Architecture string   `json:"architecture"`
	Type         string   `json:"type"`
	Size         int64    `json:"size"`
	CreatedAt    string   `json:"created_at"`
	UploadedAt   string   `json:"uploaded_at"`
	LastUsedAt   string   `json:"last_used_at"`
	AutoUpdate   bool     `json:"auto_update"`
	Cached       bool     `json:"cached"`
	Public       bool     `json:"public"`
}

func propStr(p map[string]interface{}, k string) string {
	if p == nil {
		return ""
	}
	if v, ok := p[k].(string); ok {
		return v
	}
	return ""
}

// ListLocalImages 节点本地镜像（实时读取 LXD）
func ListLocalImages(ctx context.Context) ([]LocalImage, error) {
	imgs, err := lxc.NewClient().ListImages(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]LocalImage, 0, len(imgs))
	for _, im := range imgs {
		li := LocalImage{Fingerprint: im.Fingerprint, OS: propStr(im.Properties, "os"), Release: propStr(im.Properties, "release"),
			Variant: propStr(im.Properties, "variant"), Description: propStr(im.Properties, "description"),
			Architecture: lxc.NormalizeArch(im.Architecture), Type: im.Type, Size: im.Size, CreatedAt: im.CreatedAt,
			UploadedAt: im.UploadedAt, LastUsedAt: im.LastUsedAt, AutoUpdate: im.AutoUpdate, Cached: im.Cached, Public: im.Public, Aliases: []string{}}
		if li.Type == "" {
			li.Type = "container"
		}
		for _, a := range im.Aliases {
			li.Aliases = append(li.Aliases, a.Name)
		}
		out = append(out, li)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UploadedAt > out[j].UploadedAt })
	return out, nil
}

var fpRe = regexp.MustCompile(`^[0-9a-f]{12,64}$`)
var aliasRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,127}$`)

// ValidFingerprint 指纹（前缀）校验
func ValidFingerprint(fp string) bool { return fpRe.MatchString(fp) }

// ValidImageAlias 别名校验
func ValidImageAlias(a string) bool {
	return aliasRe.MatchString(a) && !strings.Contains(a, "..") && !strings.HasSuffix(a, "/")
}

func findLocalImage(ctx context.Context, fp string) (*LocalImage, []LocalImage, error) {
	list, err := ListLocalImages(ctx)
	if err != nil {
		return nil, nil, err
	}
	var hit *LocalImage
	for i := range list {
		if strings.HasPrefix(list[i].Fingerprint, fp) {
			if hit != nil {
				return nil, list, fmt.Errorf("指纹前缀 %s 匹配多个镜像，请提供更长的指纹", fp)
			}
			hit = &list[i]
		}
	}
	if hit == nil {
		return nil, list, fmt.Errorf("镜像不存在")
	}
	return hit, list, nil
}

// syncTemplatesQuiet 镜像变更后同步模板表（商品/创建容器的镜像下拉读取该表）
func syncTemplatesQuiet(ctx context.Context) {
	if db.DB == nil {
		return
	}
	if _, _, _, err := NewTemplateService().SyncFromLXD(ctx); err != nil {
		logger.Warn("同步镜像模板失败: %v", err)
	}
}

// DeleteLocalImage 删除节点本地镜像
func DeleteLocalImage(ctx context.Context, fp string) error {
	if !ValidFingerprint(fp) {
		return fmt.Errorf("指纹无效")
	}
	img, _, err := findLocalImage(ctx, fp)
	if err != nil {
		return err
	}
	if err := lxc.NewClient().DeleteImage(ctx, img.Fingerprint); err != nil {
		return err
	}
	syncTemplatesQuiet(ctx)
	return nil
}

// SetImageAliases 把镜像别名设置为给定列表（增删差异）；别名被其它镜像占用时报错
func SetImageAliases(ctx context.Context, fp string, aliases []string) ([]string, error) {
	if !ValidFingerprint(fp) {
		return nil, fmt.Errorf("指纹无效")
	}
	want := []string{}
	seen := map[string]bool{}
	for _, a := range aliases {
		a = strings.TrimSpace(a)
		if a == "" || seen[a] {
			continue
		}
		if !ValidImageAlias(a) {
			return nil, fmt.Errorf("别名 %q 无效（字母数字开头，可含 . _ / -，最长 128）", a)
		}
		seen[a] = true
		want = append(want, a)
	}
	if len(want) > 20 {
		return nil, fmt.Errorf("别名最多 20 个")
	}
	img, all, err := findLocalImage(ctx, fp)
	if err != nil {
		return nil, err
	}
	owner := map[string]string{}
	for _, im := range all {
		for _, a := range im.Aliases {
			owner[a] = im.Fingerprint
		}
	}
	for _, a := range want {
		if o, ok := owner[a]; ok && o != img.Fingerprint {
			return nil, fmt.Errorf("别名 %s 已被镜像 %s 使用", a, o[:12])
		}
	}
	cli := lxc.NewClient()
	have := map[string]bool{}
	for _, a := range img.Aliases {
		have[a] = true
	}
	for _, a := range img.Aliases {
		if !seen[a] {
			if err := cli.DeleteImageAlias(ctx, a); err != nil {
				return nil, err
			}
		}
	}
	for _, a := range want {
		if !have[a] {
			if err := cli.CreateImageAlias(ctx, a, img.Fingerprint); err != nil {
				return nil, err
			}
		}
	}
	syncTemplatesQuiet(ctx)
	return want, nil
}

// HostArch 节点架构（lxdapi 与 LXD 同机运行）
func HostArch() string { return lxc.NormalizeArch(runtime.GOARCH) }

// ---------------- 远程目录 ----------------

// RemoteImage 远程镜像目录条目
type RemoteImage struct {
	Key          string   `json:"key"`
	OS           string   `json:"os"`
	Release      string   `json:"release"`
	ReleaseTitle string   `json:"release_title"`
	Arch         string   `json:"arch"`
	Variant      string   `json:"variant"`
	Aliases      []string `json:"aliases"`
	Version      string   `json:"version"`
	Fingerprint  string   `json:"fingerprint"`
	Size         int64    `json:"size"`
	Method       string   `json:"method"` // lxd=由 LXD 直接拉取；files=节点下载文件后导入
	MetaPath     string   `json:"meta_path,omitempty"`
	RootfsPath   string   `json:"rootfs_path,omitempty"`
	MetaSHA256   string   `json:"meta_sha256,omitempty"`
	RootfsSHA256 string   `json:"rootfs_sha256,omitempty"`
}

type catalogCacheEntry struct {
	at    time.Time
	items []RemoteImage
}

var catalogCache = struct {
	sync.Mutex
	m map[string]catalogCacheEntry
}{m: map[string]catalogCacheEntry{}}

var remoteHTTP = &http.Client{Timeout: 90 * time.Second}

// FetchRemoteCatalog 读取远程镜像目录（缓存 30 分钟）
func FetchRemoteCatalog(ctx context.Context, server, protocol string, refresh bool) ([]RemoteImage, error) {
	server, err := ValidateRemoteServer(server, protocol)
	if err != nil {
		return nil, err
	}
	key := protocol + "|" + server
	catalogCache.Lock()
	if e, ok := catalogCache.m[key]; ok && !refresh && time.Since(e.at) < 30*time.Minute {
		catalogCache.Unlock()
		return e.items, nil
	}
	catalogCache.Unlock()
	var items []RemoteImage
	if protocol == "lxd" {
		items, err = fetchLXDCatalog(ctx, server)
	} else {
		items, err = fetchSimplestreams(ctx, server)
	}
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].OS != items[j].OS {
			return strings.ToLower(items[i].OS) < strings.ToLower(items[j].OS)
		}
		if items[i].Release != items[j].Release {
			return items[i].Release > items[j].Release
		}
		return items[i].Key < items[j].Key
	})
	catalogCache.Lock()
	catalogCache.m[key] = catalogCacheEntry{at: time.Now(), items: items}
	catalogCache.Unlock()
	return items, nil
}

func httpGetJSON(ctx context.Context, client *http.Client, u string, v interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "lxdcloud/"+appVersion())
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("访问 %s 失败: %v", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("访问 %s 返回 HTTP %d", u, resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 128<<20)).Decode(v)
}

// AppVersion 由 main 设置，用于 User-Agent
var AppVersion = ""

func appVersion() string {
	if AppVersion == "" {
		return "dev"
	}
	return AppVersion
}

type ssItem struct {
	FType            string `json:"ftype"`
	Path             string `json:"path"`
	Size             int64  `json:"size"`
	SHA256           string `json:"sha256"`
	CombinedSHA256   string `json:"combined_sha256"`
	CombinedSquashfs string `json:"combined_squashfs_sha256"`
	CombinedRootxz   string `json:"combined_rootxz_sha256"`
}

type ssProduct struct {
	Aliases      string                     `json:"aliases"`
	Arch         string                     `json:"arch"`
	OS           string                     `json:"os"`
	Release      string                     `json:"release"`
	ReleaseTitle string                     `json:"release_title"`
	Version      string                     `json:"version"`
	Variant      string                     `json:"variant"`
	Versions     map[string]json.RawMessage `json:"versions"`
}

func fetchSimplestreams(ctx context.Context, server string) ([]RemoteImage, error) {
	var idx struct {
		Index map[string]struct {
			Datatype string `json:"datatype"`
			Path     string `json:"path"`
		} `json:"index"`
	}
	if err := httpGetJSON(ctx, remoteHTTP, server+"/streams/v1/index.json", &idx); err != nil {
		return nil, err
	}
	var out []RemoteImage
	found := false
	for _, e := range idx.Index {
		if e.Datatype != "image-downloads" || e.Path == "" || strings.Contains(e.Path, "..") {
			continue
		}
		found = true
		var doc struct {
			Products map[string]ssProduct `json:"products"`
		}
		if err := httpGetJSON(ctx, remoteHTTP, server+"/"+strings.TrimLeft(e.Path, "/"), &doc); err != nil {
			return nil, err
		}
		out = append(out, ParseSimplestreamsProducts(doc.Products)...)
	}
	if !found {
		return nil, fmt.Errorf("该地址不是 simplestreams 镜像源（index.json 中无 image-downloads）")
	}
	return out, nil
}

// ParseSimplestreamsProducts 解析 simplestreams products（取每个产品的最新版本，只保留容器镜像）
func ParseSimplestreamsProducts(products map[string]ssProduct) []RemoteImage {
	var out []RemoteImage
	for key, p := range products {
		if len(p.Versions) == 0 {
			continue
		}
		vers := make([]string, 0, len(p.Versions))
		for v := range p.Versions {
			vers = append(vers, v)
		}
		sort.Strings(vers)
		// 从最新版本往前找第一个有容器镜像的版本
		for i := len(vers) - 1; i >= 0; i-- {
			var ver struct {
				Items map[string]ssItem `json:"items"`
			}
			if json.Unmarshal(p.Versions[vers[i]], &ver) != nil || len(ver.Items) == 0 {
				continue
			}
			var lxdMeta, incusMeta, squash, rootxz *ssItem
			for k := range ver.Items {
				it := ver.Items[k]
				switch it.FType {
				case "lxd.tar.xz":
					lxdMeta = &it
				case "incus.tar.xz":
					incusMeta = &it
				case "squashfs":
					squash = &it
				case "root.tar.xz":
					rootxz = &it
				}
			}
			ri := RemoteImage{Key: key, OS: p.OS, Release: p.Release, ReleaseTitle: p.ReleaseTitle, Arch: lxc.NormalizeArch(p.Arch),
				Variant: p.Variant, Version: vers[i], Aliases: []string{}}
			if ri.Release == "" {
				ri.Release = p.Version
			}
			for _, a := range strings.Split(p.Aliases, ",") {
				if a = strings.TrimSpace(a); a != "" {
					ri.Aliases = append(ri.Aliases, a)
				}
			}
			var meta, rootfs *ssItem
			switch {
			case lxdMeta != nil && (squash != nil || rootxz != nil):
				ri.Method, meta = "lxd", lxdMeta
			case incusMeta != nil && (squash != nil || rootxz != nil):
				ri.Method, meta = "files", incusMeta
			default:
				continue // 只有虚拟机镜像或格式未知
			}
			if squash != nil && meta.CombinedSquashfs != "" {
				rootfs, ri.Fingerprint = squash, meta.CombinedSquashfs
			} else if rootxz != nil {
				rootfs = rootxz
				ri.Fingerprint = meta.CombinedRootxz
				if ri.Fingerprint == "" {
					ri.Fingerprint = meta.CombinedSHA256
				}
			} else {
				rootfs, ri.Fingerprint = squash, meta.CombinedSHA256
			}
			if ri.Fingerprint == "" {
				continue
			}
			ri.Size = meta.Size + rootfs.Size
			ri.MetaPath, ri.RootfsPath = meta.Path, rootfs.Path
			ri.MetaSHA256, ri.RootfsSHA256 = meta.SHA256, rootfs.SHA256
			out = append(out, ri)
			break
		}
	}
	return out
}

func lxdRemoteClient() *http.Client {
	// LXD 公共远程常为自签证书：仅用于浏览公开镜像列表；下载时把证书原样交给 LXD 校验
	return &http.Client{Timeout: 60 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
}

func fetchLXDCatalog(ctx context.Context, server string) ([]RemoteImage, error) {
	var resp struct {
		Type     string `json:"type"`
		Error    string `json:"error"`
		Metadata []struct {
			Fingerprint  string                 `json:"fingerprint"`
			Architecture string                 `json:"architecture"`
			Size         int64                  `json:"size"`
			Type         string                 `json:"type"`
			Public       bool                   `json:"public"`
			CreatedAt    string                 `json:"created_at"`
			Properties   map[string]interface{} `json:"properties"`
			Aliases      []struct {
				Name string `json:"name"`
			} `json:"aliases"`
		} `json:"metadata"`
	}
	if err := httpGetJSON(ctx, lxdRemoteClient(), server+"/1.0/images?recursion=1", &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("LXD 远程返回错误: %s", resp.Error)
	}
	var out []RemoteImage
	for _, m := range resp.Metadata {
		if m.Type != "" && m.Type != "container" {
			continue
		}
		ri := RemoteImage{Key: m.Fingerprint, OS: propStr(m.Properties, "os"), Release: propStr(m.Properties, "release"),
			ReleaseTitle: propStr(m.Properties, "description"), Arch: lxc.NormalizeArch(m.Architecture), Variant: propStr(m.Properties, "variant"),
			Version: m.CreatedAt, Fingerprint: m.Fingerprint, Size: m.Size, Method: "lxd", Aliases: []string{}}
		for _, a := range m.Aliases {
			ri.Aliases = append(ri.Aliases, a.Name)
		}
		out = append(out, ri)
	}
	return out, nil
}

// fetchServerCertPEM 取 LXD 远程的 TLS 证书（交给 LXD 作为 source.certificate）
func fetchServerCertPEM(server string) string {
	u, err := url.Parse(server)
	if err != nil {
		return ""
	}
	host := u.Host
	if u.Port() == "" {
		host += ":443"
	}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", host, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return ""
	}
	defer conn.Close()
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return ""
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certs[0].Raw}))
}

// ---------------- 下载 / 导入 ----------------

// ImageDownloadRequest 从远程下载镜像到节点
type ImageDownloadRequest struct {
	Server         string `json:"server"`
	Protocol       string `json:"protocol"`
	Fingerprint    string `json:"fingerprint"`
	RemoteAlias    string `json:"remote_alias"` // 自动更新时按远程别名拉取
	Method         string `json:"method"`       // lxd / files
	MetaPath       string `json:"meta_path"`
	RootfsPath     string `json:"rootfs_path"`
	MetaSHA256     string `json:"meta_sha256"`
	RootfsSHA256   string `json:"rootfs_sha256"`
	Alias          string `json:"alias"` // 本地别名
	AutoUpdate     bool   `json:"auto_update"`
	OverwriteAlias bool   `json:"overwrite_alias"`
	Label          string `json:"label"`
}

var relPathRe = regexp.MustCompile(`^[A-Za-z0-9._:+~/@-]+$`)

func validRelPath(p string) bool {
	return p != "" && len(p) < 512 && relPathRe.MatchString(p) && !strings.Contains(p, "..") && !strings.HasPrefix(p, "/")
}

// Validate 参数校验与规范化
func (r *ImageDownloadRequest) Validate() error {
	srv, err := ValidateRemoteServer(r.Server, r.Protocol)
	if err != nil {
		return err
	}
	r.Server = srv
	r.Fingerprint = strings.ToLower(strings.TrimSpace(r.Fingerprint))
	r.Alias = strings.TrimSpace(r.Alias)
	r.RemoteAlias = strings.TrimSpace(r.RemoteAlias)
	if r.Method == "" {
		r.Method = "lxd"
	}
	if r.Method != "lxd" && r.Method != "files" {
		return fmt.Errorf("下载方式只能是 lxd 或 files")
	}
	if r.Fingerprint != "" && !ValidFingerprint(r.Fingerprint) {
		return fmt.Errorf("指纹无效")
	}
	if r.RemoteAlias != "" && !ValidImageAlias(r.RemoteAlias) {
		return fmt.Errorf("远程别名无效")
	}
	if r.Alias != "" && !ValidImageAlias(r.Alias) {
		return fmt.Errorf("本地别名无效（字母数字开头，可含 . _ / -，最长 128）")
	}
	if r.Method == "lxd" {
		if r.Fingerprint == "" && r.RemoteAlias == "" {
			return fmt.Errorf("缺少镜像指纹或远程别名")
		}
		if r.AutoUpdate && r.RemoteAlias == "" {
			return fmt.Errorf("自动更新需要远程别名")
		}
	} else {
		if r.Protocol != "simplestreams" {
			return fmt.Errorf("文件方式只支持 simplestreams 源")
		}
		if !validRelPath(r.MetaPath) || !validRelPath(r.RootfsPath) {
			return fmt.Errorf("镜像文件路径无效")
		}
		for _, h := range []string{r.MetaSHA256, r.RootfsSHA256} {
			if h != "" && !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(h) {
				return fmt.Errorf("sha256 无效")
			}
		}
		r.AutoUpdate = false
	}
	if r.Label == "" {
		r.Label = r.Alias
		if r.Label == "" {
			r.Label = r.RemoteAlias
		}
		if r.Label == "" && len(r.Fingerprint) >= 12 {
			r.Label = r.Fingerprint[:12]
		}
	}
	if len([]rune(r.Label)) > 100 {
		r.Label = string([]rune(r.Label)[:100])
	}
	return nil
}

// ImageImportURLRequest 从 URL 导入 tar 镜像
type ImageImportURLRequest struct {
	URL            string `json:"url"`        // 统一格式 tar(.gz/.xz) 或拆分格式的 metadata
	RootfsURL      string `json:"rootfs_url"` // 拆分格式的 rootfs（squashfs/tar.xz），可选
	Alias          string `json:"alias"`
	OverwriteAlias bool   `json:"overwrite_alias"`
}

func validDownloadURL(s string) error {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return fmt.Errorf("URL 需为 http(s):// 开头的有效地址")
	}
	return nil
}

// Validate 参数校验
func (r *ImageImportURLRequest) Validate() error {
	r.URL, r.RootfsURL, r.Alias = strings.TrimSpace(r.URL), strings.TrimSpace(r.RootfsURL), strings.TrimSpace(r.Alias)
	if err := validDownloadURL(r.URL); err != nil {
		return err
	}
	if r.RootfsURL != "" {
		if err := validDownloadURL(r.RootfsURL); err != nil {
			return fmt.Errorf("rootfs %v", err)
		}
	}
	if r.Alias == "" {
		return fmt.Errorf("请填写本地别名")
	}
	if !ValidImageAlias(r.Alias) {
		return fmt.Errorf("本地别名无效（字母数字开头，可含 . _ / -，最长 128）")
	}
	return nil
}

// checkAliasFree 别名已被占用且不允许覆盖时提前报错
func checkAliasFree(ctx context.Context, alias string, overwrite bool) error {
	if alias == "" || overwrite {
		return nil
	}
	list, err := ListLocalImages(ctx)
	if err != nil {
		return err
	}
	for _, im := range list {
		for _, a := range im.Aliases {
			if a == alias {
				return fmt.Errorf("别名 %s 已被镜像 %s 使用（可勾选「覆盖同名别名」用新镜像替换）", alias, im.Fingerprint[:12])
			}
		}
	}
	return nil
}

// StartImageDownload 提交下载作业
func StartImageDownload(ctx context.Context, req ImageDownloadRequest) (*BgJob, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if !lxc.BinaryAvailable() {
		return nil, lxc.ErrLXDNotInstalled
	}
	if err := checkAliasFree(ctx, req.Alias, req.OverwriteAlias); err != nil {
		return nil, err
	}
	b, _ := json.Marshal(req)
	var params map[string]interface{}
	json.Unmarshal(b, &params)
	return submitBgJob("image_download", req.Label, params)
}

// StartImageImportURL 提交 URL 导入作业
func StartImageImportURL(ctx context.Context, req ImageImportURLRequest) (*BgJob, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if !lxc.BinaryAvailable() {
		return nil, lxc.ErrLXDNotInstalled
	}
	if err := checkAliasFree(ctx, req.Alias, req.OverwriteAlias); err != nil {
		return nil, err
	}
	b, _ := json.Marshal(req)
	var params map[string]interface{}
	json.Unmarshal(b, &params)
	return submitBgJob("image_import", req.Alias, params)
}

func init() {
	bgJobRunners["image_download"] = runImageDownload
	bgJobRunners["image_import"] = runImageImportURL
}

func jobMsg(id string, progress int, msg string) {
	updateJob(id, func(j *BgJob) {
		if progress >= 0 {
			j.Progress = progress
		}
		j.Message = msg
	})
}

var pctRe = regexp.MustCompile(`(\d{1,3})%`)

func runImageDownload(ctx context.Context, jobID, paramsJSON string) error {
	var req ImageDownloadRequest
	if err := json.Unmarshal([]byte(paramsJSON), &req); err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return err
	}
	updateJob(jobID, func(j *BgJob) { j.Alias = req.Alias; j.Fingerprint = req.Fingerprint })
	var fp string
	var err error
	if req.Method == "files" {
		fp, err = downloadAndImport(ctx, jobID, req.Server+"/"+req.MetaPath, req.MetaSHA256, req.Server+"/"+req.RootfsPath, req.RootfsSHA256)
	} else {
		fp, err = pullViaLXD(ctx, jobID, req)
	}
	if err != nil {
		return err
	}
	return finishImage(ctx, jobID, fp, req.Alias, req.OverwriteAlias)
}

func runImageImportURL(ctx context.Context, jobID, paramsJSON string) error {
	var req ImageImportURLRequest
	if err := json.Unmarshal([]byte(paramsJSON), &req); err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return err
	}
	updateJob(jobID, func(j *BgJob) { j.Alias = req.Alias })
	fp, err := downloadAndImport(ctx, jobID, req.URL, "", req.RootfsURL, "")
	if err != nil {
		return err
	}
	return finishImage(ctx, jobID, fp, req.Alias, req.OverwriteAlias)
}

// finishImage 设置别名并同步模板表
func finishImage(ctx context.Context, jobID, fp, alias string, overwrite bool) error {
	updateJob(jobID, func(j *BgJob) { j.Fingerprint = fp })
	if alias != "" {
		jobMsg(jobID, 99, "设置别名 "+alias)
		list, err := ListLocalImages(ctx)
		if err != nil {
			return err
		}
		cli := lxc.NewClient()
		already := false
		for _, im := range list {
			for _, a := range im.Aliases {
				if a != alias {
					continue
				}
				if im.Fingerprint == fp {
					already = true
				} else if overwrite {
					if err := cli.DeleteImageAlias(ctx, alias); err != nil {
						return err
					}
				} else {
					return fmt.Errorf("镜像已下载（%s），但别名 %s 已被镜像 %s 使用，未设置别名", fp[:12], alias, im.Fingerprint[:12])
				}
			}
		}
		if !already {
			if err := cli.CreateImageAlias(ctx, alias, fp); err != nil {
				return err
			}
		}
	}
	syncTemplatesQuiet(ctx)
	jobMsg(jobID, 100, "完成："+fp[:12]+ifStr(alias != "", "（别名 "+alias+"）", ""))
	return nil
}

func ifStr(c bool, a, b string) string {
	if c {
		return a
	}
	return b
}

// pullViaLXD 由 LXD 直接从远程拉取（POST /1.0/images，source.type=image），轮询操作进度
func pullViaLXD(ctx context.Context, jobID string, req ImageDownloadRequest) (string, error) {
	src := map[string]interface{}{"type": "image", "mode": "pull", "server": req.Server, "protocol": req.Protocol, "image_type": "container"}
	if req.AutoUpdate || req.Fingerprint == "" {
		src["alias"] = req.RemoteAlias
	} else {
		src["fingerprint"] = req.Fingerprint
	}
	if req.Protocol == "lxd" {
		if cert := fetchServerCertPEM(req.Server); cert != "" {
			src["certificate"] = cert
		}
	}
	body, _ := json.Marshal(map[string]interface{}{"source": src, "auto_update": req.AutoUpdate, "public": false})
	cli := lxc.NewClient()
	jobMsg(jobID, 0, "提交下载到 LXD")
	r, err := cli.Query(ctx, "POST", "/1.0/images", string(body))
	if err != nil {
		return "", fmt.Errorf("LXD 拒绝下载请求: %v", err)
	}
	if r.Error != "" {
		return "", fmt.Errorf("LXD 返回错误: %s", r.Error)
	}
	opID := r.OperationID()
	if opID == "" {
		if req.Fingerprint != "" {
			return req.Fingerprint, nil
		}
		return "", fmt.Errorf("LXD 未返回操作 ID")
	}
	misses := 0
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("下载超时或被取消")
		case <-time.After(2 * time.Second):
		}
		op, err := cli.GetOperation(ctx, opID)
		if err != nil {
			// 操作结束后一段时间会被 LXD 清理：以本地是否已有镜像判断
			misses++
			if req.Fingerprint != "" {
				if img, _, e := findLocalImage(ctx, req.Fingerprint[:12]); e == nil {
					return img.Fingerprint, nil
				}
			}
			if misses >= 5 {
				return "", fmt.Errorf("查询下载进度失败: %v", err)
			}
			continue
		}
		misses = 0
		switch op.Status {
		case "Success":
			if fp, ok := op.Metadata["fingerprint"].(string); ok && fp != "" {
				return fp, nil
			}
			if req.Fingerprint != "" {
				if img, _, e := findLocalImage(ctx, req.Fingerprint[:12]); e == nil {
					return img.Fingerprint, nil
				}
			}
			return "", fmt.Errorf("下载完成但未获取到镜像指纹")
		case "Failure", "Cancelled":
			if op.Err == "" {
				op.Err = op.Status
			}
			return "", fmt.Errorf("下载失败: %s", op.Err)
		default:
			msg := ""
			if v, ok := op.Metadata["download_progress"].(string); ok {
				msg = v
			}
			p := -1
			if m := pctRe.FindStringSubmatch(msg); m != nil {
				p, _ = strconv.Atoi(m[1])
				if p > 98 {
					p = 98
				}
			}
			if msg == "" {
				msg = "下载中"
			}
			jobMsg(jobID, p, msg)
		}
	}
}

// downloadAndImport 节点下载镜像文件（校验 sha256）后 lxc image import
func downloadAndImport(ctx context.Context, jobID, metaURL, metaSHA, rootfsURL, rootfsSHA string) (string, error) {
	dir := core.ImageTmpDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建临时目录失败: %v", err)
	}
	tmp, err := os.MkdirTemp(dir, "img-")
	if err != nil {
		return "", fmt.Errorf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmp)
	metaFile := filepath.Join(tmp, "meta-"+safeFileName(metaURL))
	if err := downloadFile(ctx, jobID, metaURL, metaFile, metaSHA, 0, ifInt(rootfsURL == "", 95, 5)); err != nil {
		return "", err
	}
	rootFile := ""
	if rootfsURL != "" {
		rootFile = filepath.Join(tmp, "rootfs-"+safeFileName(rootfsURL))
		if err := downloadFile(ctx, jobID, rootfsURL, rootFile, rootfsSHA, 5, 95); err != nil {
			return "", err
		}
	}
	jobMsg(jobID, 96, "导入到 LXD")
	return lxc.NewClient().ImportImageFiles(ctx, metaFile, rootFile)
}

func ifInt(c bool, a, b int) int {
	if c {
		return a
	}
	return b
}

func safeFileName(u string) string {
	n := path.Base(strings.SplitN(u, "?", 2)[0])
	n = regexp.MustCompile(`[^A-Za-z0-9._-]`).ReplaceAllString(n, "_")
	if n == "" || n == "." || n == "_" {
		n = "file"
	}
	if len(n) > 80 {
		n = n[len(n)-80:]
	}
	return n
}

var downloadHTTP = &http.Client{} // 大文件不设整体超时，由 ctx 控制

func downloadFile(ctx context.Context, jobID, u, dest, wantSHA string, pFrom, pTo int) error {
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "lxdcloud/"+appVersion())
	resp, err := downloadHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("下载 %s 失败: %v", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("下载 %s 返回 HTTP %d", u, resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	total := resp.ContentLength
	var done int64
	buf := make([]byte, 256<<10)
	last := time.Now()
	name := path.Base(strings.SplitN(u, "?", 2)[0])
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				return fmt.Errorf("写入失败（磁盘空间不足？）: %v", err)
			}
			h.Write(buf[:n])
			done += int64(n)
			if time.Since(last) > time.Second {
				last = time.Now()
				p := -1
				msg := fmt.Sprintf("下载 %s %.1f MB", name, float64(done)/1048576)
				if total > 0 {
					p = pFrom + int(float64(pTo-pFrom)*float64(done)/float64(total))
					msg += fmt.Sprintf(" / %.1f MB", float64(total)/1048576)
				}
				jobMsg(jobID, p, msg)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return fmt.Errorf("下载中断: %v", rerr)
		}
	}
	if wantSHA != "" && hex.EncodeToString(h.Sum(nil)) != wantSHA {
		return fmt.Errorf("%s 校验失败（sha256 不一致）", name)
	}
	jobMsg(jobID, pTo, "已下载 "+name)
	return nil
}
