package service

// 节点本地包缓存管理（v1.1.22，仅管理端；主控按 node_id 转发到节点执行）

import (
	"context"
	"encoding/json"
	"fmt"
	"lxdapi/internal/core"
	"lxdapi/internal/lxc"
	"path/filepath"
	"strings"
)

// PkgCacheStatus 节点包缓存状态
type PkgCacheStatus struct {
	Dir             string               `json:"dir"`
	AbsDir          string               `json:"abs_dir"`
	Settings        lxc.PkgCacheSettings `json:"settings"`
	DefaultTimeout  int                  `json:"default_online_timeout"`
	Disabled        bool                 `json:"disabled"`
	LXDAvailable    bool                 `json:"lxd_available"`
	Entries         []lxc.PkgCacheEntry  `json:"entries"`
	Images          []LocalImage         `json:"images"`
	Jobs            []BgJob              `json:"jobs"`
	Running         *BgJob               `json:"running"`
	DefaultPackages []string             `json:"default_packages"`
	Suggestions     []string             `json:"suggestions"`
	HostArch        string               `json:"host_arch"`
}

// GetPkgCacheStatus 汇总状态
func GetPkgCacheStatus(ctx context.Context) PkgCacheStatus {
	st := PkgCacheStatus{Dir: core.PkgCacheDir(), Settings: lxc.LoadPkgCacheSettings(), DefaultTimeout: core.PkgCacheOnlineTimeout(),
		LXDAvailable: LocalLXDAvailable(), Entries: lxc.ListPkgCache(), Jobs: ListJobs("pkgcache_sync"), Running: jobRunning("pkgcache_sync"),
		DefaultPackages: lxc.DefaultPkgCachePackages, HostArch: HostArch(),
		Suggestions: []string{"openssh", "wget", "curl", "ca-certificates", "sudo", "bash", "nano", "vim", "tar", "unzip", "htop"}}
	if abs, err := filepath.Abs(st.Dir); err == nil {
		st.AbsDir = abs
	}
	st.Disabled = core.GlobalConfig != nil && core.GlobalConfig.PkgCache.Disabled
	if st.Entries == nil {
		st.Entries = []lxc.PkgCacheEntry{}
	}
	if st.Jobs == nil {
		st.Jobs = []BgJob{}
	}
	st.Images = []LocalImage{}
	if st.LXDAvailable {
		if imgs, err := ListLocalImages(ctx); err == nil {
			for _, im := range imgs {
				if im.Type == "" || im.Type == "container" {
					st.Images = append(st.Images, im)
				}
			}
		}
	}
	return st
}

// PkgCacheSyncRequest 同步请求
type PkgCacheSyncRequest struct {
	Images   []string `json:"images"`   // 镜像指纹或别名；为空且 all=true 时取全部本地容器镜像（按系统/版本/架构去重）
	All      bool     `json:"all"`
	Packages []string `json:"packages"` // 为空用设置中的列表
}

// StartPkgCacheSync 提交同步作业（同一节点同时只允许一个）
func StartPkgCacheSync(ctx context.Context, req PkgCacheSyncRequest) (*BgJob, error) {
	if !LocalLXDAvailable() {
		return nil, fmt.Errorf("本机未连接 LXD，无法同步包缓存（请在节点上操作）")
	}
	if j := jobRunning("pkgcache_sync"); j != nil {
		return nil, fmt.Errorf("已有同步任务在进行中")
	}
	for _, p := range req.Packages {
		if !lxc.ValidPkgName(strings.ToLower(strings.TrimSpace(p))) {
			return nil, fmt.Errorf("包名 %q 无效", p)
		}
	}
	imgs, err := ListLocalImages(ctx)
	if err != nil {
		return nil, err
	}
	var targets []string
	if len(req.Images) == 0 {
		if !req.All {
			return nil, fmt.Errorf("请选择镜像")
		}
		seen := map[string]bool{}
		for _, im := range imgs {
			if im.Type != "" && im.Type != "container" {
				continue
			}
			k := strings.ToLower(im.OS) + "/" + strings.ToLower(im.Release) + "/" + im.Architecture
			if im.OS != "" && seen[k] {
				continue
			}
			seen[k] = true
			targets = append(targets, im.Fingerprint)
		}
		if len(targets) == 0 {
			return nil, fmt.Errorf("节点没有本地容器镜像，请先在「镜像管理」下载")
		}
	} else {
		if len(req.Images) > 30 {
			return nil, fmt.Errorf("一次最多同步 30 个镜像")
		}
		for _, ref := range req.Images {
			ref = strings.TrimSpace(ref)
			found := ""
			for _, im := range imgs {
				if (ValidFingerprint(ref) && strings.HasPrefix(im.Fingerprint, ref)) || containsStr(im.Aliases, ref) {
					found = im.Fingerprint
					break
				}
			}
			if found == "" {
				return nil, fmt.Errorf("节点上没有镜像 %q", ref)
			}
			targets = append(targets, found)
		}
	}
	pk := req.Packages
	if len(pk) == 0 {
		pk = lxc.LoadPkgCacheSettings().Packages
	}
	params := map[string]interface{}{"images": targets, "packages": lxc.NormalizePkgList(pk)}
	return submitBgJob("pkgcache_sync", fmt.Sprintf("%d 个镜像", len(targets)), params)
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func init() {
	bgJobRunners["pkgcache_sync"] = runPkgCacheSync
}

func runPkgCacheSync(ctx context.Context, jobID, paramsJSON string) error {
	var p struct {
		Images   []string `json:"images"`
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &p); err != nil {
		return err
	}
	cli := lxc.NewClient()
	var okKeys, fails []string
	done := map[string]bool{}
	for i, img := range p.Images {
		if !ValidFingerprint(img) {
			fails = append(fails, img+": 指纹无效")
			continue
		}
		short := img
		if len(short) > 12 {
			short = short[:12]
		}
		base := i * 100 / len(p.Images)
		m, err := cli.SyncPkgCacheFromImage(ctx, img, p.Packages, func(msg string) {
			jobMsg(jobID, base, fmt.Sprintf("[%d/%d %s] %s", i+1, len(p.Images), short, msg))
		})
		if err != nil {
			fails = append(fails, short+": "+err.Error())
			continue
		}
		key := m.Distro + "/" + m.Version + "/" + m.Arch
		if !done[key] {
			done[key] = true
			okKeys = append(okKeys, fmt.Sprintf("%s(%d 个包)", key, m.Files))
		}
	}
	summary := "已同步: " + strings.Join(okKeys, "，")
	if len(okKeys) == 0 {
		summary = "没有成功同步的镜像"
	}
	if len(fails) > 0 {
		summary += "；失败: " + strings.Join(fails, "；")
	}
	jobMsg(jobID, 100, summary)
	if len(okKeys) == 0 {
		return fmt.Errorf("%s", summary)
	}
	return nil
}
