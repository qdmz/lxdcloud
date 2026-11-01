package lxc

// 节点本地常用包缓存（v1.1.22）
//
// 解决容器无法联网时装不上 openssh-server（含 sftp-server）/wget/curl，导致 SSH/SFTP 不可用的问题：
//   - 缓存目录 <dir>/<发行版>/<版本>/<架构>/，存放 .apk/.deb/.rpm 及 manifest.json；
//   - 同步由节点宿主机发起：用目标镜像起一个临时容器（走宿主机网络出口），在容器内用发行版自己的包管理器
//     下载完整依赖闭包（apk fetch --recursive / apt-get download 依赖闭包 / dnf download --resolve），
//     再 lxc file pull 回宿主机缓存目录。这样不要求宿主机与容器同发行版，也不需要宿主机安装 apk/dnf；
//   - 开通/重装/改密后：先在线安装（带超时），失败则把缓存的包 lxc file push 进容器离线安装，
//     再配置 sshd（PermitRootLogin yes、PasswordAuthentication yes、ssh-keygen -A、开机自启并启动）。
// 全程失败只记 Warn 日志，不影响开通。

import (
	"context"
	"encoding/json"
	"fmt"
	"lxdapi/internal/core"
	"lxdapi/pkg/logger"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultPkgCachePackages 默认预装包（逻辑名）
var DefaultPkgCachePackages = []string{"openssh", "wget", "curl"}

// PkgCacheSettings 存于 <dir>/settings.json（节点本地，可在后台修改）
type PkgCacheSettings struct {
	Packages      []string `json:"packages"`       // 需要预装的包（逻辑名；openssh 总是包含）
	OnlineTimeout int      `json:"online_timeout"` // 在线安装超时秒数，0 使用配置文件默认值
	AutoInstall   *bool    `json:"auto_install"`   // 开通/改密后是否自动安装（nil=true）
}

// PkgCacheManifest 每个缓存目录的说明
type PkgCacheManifest struct {
	Distro   string   `json:"distro"`
	Version  string   `json:"version"`
	Arch     string   `json:"arch"`
	PM       string   `json:"pm"`
	Packages []string `json:"packages"`
	Files    int      `json:"files"`
	Size     int64    `json:"size"`
	Image    string   `json:"image"`
	SyncedAt string   `json:"synced_at"`
	Log      string   `json:"log,omitempty"`
}

var pkgNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9.+_-]{0,63}$`)

// ValidPkgName 包名校验（防止注入到容器内 shell）
func ValidPkgName(n string) bool { return pkgNameRe.MatchString(n) }

var pkgSettingsMu sync.Mutex

func pkgSettingsPath() string { return filepath.Join(core.PkgCacheDir(), "settings.json") }

// LoadPkgCacheSettings 读取设置（不存在时返回默认值）
func LoadPkgCacheSettings() PkgCacheSettings {
	pkgSettingsMu.Lock()
	defer pkgSettingsMu.Unlock()
	s := PkgCacheSettings{}
	if b, err := os.ReadFile(pkgSettingsPath()); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	s.Packages = NormalizePkgList(s.Packages)
	return s
}

// SavePkgCacheSettings 保存设置
func SavePkgCacheSettings(s PkgCacheSettings) error {
	s.Packages = NormalizePkgList(s.Packages)
	if s.OnlineTimeout < 0 || s.OnlineTimeout > 1800 {
		return fmt.Errorf("在线安装超时需在 0~1800 秒之间")
	}
	pkgSettingsMu.Lock()
	defer pkgSettingsMu.Unlock()
	if err := os.MkdirAll(core.PkgCacheDir(), 0755); err != nil {
		return fmt.Errorf("创建缓存目录失败: %v", err)
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	tmp := pkgSettingsPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, pkgSettingsPath())
}

// NormalizePkgList 去重、去非法名，确保包含 openssh；为空时使用默认列表
func NormalizePkgList(in []string) []string {
	out := []string{"openssh"}
	seen := map[string]bool{"openssh": true}
	src := in
	if len(src) == 0 {
		src = DefaultPkgCachePackages
	}
	for _, p := range src {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "openssh-server" || p == "sshd" || p == "ssh" {
			p = "openssh"
		}
		if p == "" || seen[p] || !ValidPkgName(p) {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// pmFamily 包管理器 → 家族
func pmFamily(pm string) string {
	switch pm {
	case "apk":
		return "apk"
	case "apt-get", "apt":
		return "apt"
	case "dnf", "yum":
		return "rpm"
	case "zypper":
		return "zypper"
	case "pacman":
		return "pacman"
	}
	return ""
}

// MapPackages 逻辑包名 → 指定包管理器下的真实包名
func MapPackages(pm string, logical []string) []string {
	fam := pmFamily(pm)
	var out []string
	seen := map[string]bool{}
	add := func(n ...string) {
		for _, x := range n {
			if !seen[x] {
				seen[x] = true
				out = append(out, x)
			}
		}
	}
	for _, l := range logical {
		switch l {
		case "openssh":
			switch fam {
			case "apk":
				// Alpine 的 sftp-server 是独立子包，必须显式安装，否则 SFTP 不可用
				add("openssh-server", "openssh-sftp-server", "openssh-keygen")
			case "apt":
				add("openssh-server", "openssh-sftp-server")
			case "rpm":
				add("openssh-server")
			case "zypper", "pacman":
				add("openssh")
			default:
				add("openssh-server")
			}
		default:
			add(l)
		}
	}
	return out
}

// ContainerOSInfo 容器内探测结果
type ContainerOSInfo struct {
	Distro  string // os-release ID（小写）
	Version string // 规范化后的版本
	Arch    string // amd64/arm64/...
	PM      string // apk/apt-get/dnf/yum/zypper/pacman
	Has     map[string]bool
}

// Key 缓存子目录 <发行版>/<版本>/<架构>
func (i ContainerOSInfo) Key() string {
	return i.Distro + "/" + i.Version + "/" + i.Arch
}

// NormalizeArch uname -m → LXD/Debian 风格架构名
func NormalizeArch(a string) string {
	switch strings.ToLower(strings.TrimSpace(a)) {
	case "x86_64", "amd64", "x64":
		return "amd64"
	case "aarch64", "arm64":
		return "arm64"
	case "armv7l", "armv7", "armhf", "armv8l":
		return "armhf"
	case "i686", "i386", "i586", "x86":
		return "i386"
	case "ppc64le", "ppc64el":
		return "ppc64el"
	case "s390x":
		return "s390x"
	case "riscv64":
		return "riscv64"
	}
	return safeSeg(strings.ToLower(a))
}

// NormalizeVersion 发行版版本号规范化（同一大版本共用一份缓存）
func NormalizeVersion(distro, ver string) string {
	d := strings.ToLower(distro)
	v := strings.Trim(strings.TrimSpace(ver), `"`)
	switch {
	case d == "arch" || d == "archlinux" || strings.Contains(d, "tumbleweed") || d == "gentoo" || d == "void":
		return "rolling"
	case v == "":
		return "rolling"
	case d == "alpine":
		parts := strings.Split(v, ".")
		if len(parts) >= 2 {
			return safeSeg(parts[0] + "." + parts[1])
		}
		return safeSeg(v)
	case d == "ubuntu" || strings.HasPrefix(d, "opensuse"):
		return safeSeg(v)
	default:
		return safeSeg(strings.Split(v, ".")[0])
	}
}

var segRe = regexp.MustCompile(`[^a-z0-9._-]`)

func safeSeg(s string) string {
	s = segRe.ReplaceAllString(strings.ToLower(s), "")
	s = strings.Trim(s, ".")
	if s == "" {
		return "unknown"
	}
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

// ParseDetectOutput 解析探测脚本输出：LXDC|id|version|arch|pm|has1,has2
func ParseDetectOutput(out string) (ContainerOSInfo, error) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "LXDC|") {
			continue
		}
		f := strings.Split(line, "|")
		if len(f) < 6 {
			break
		}
		info := ContainerOSInfo{Distro: safeSeg(f[1]), Arch: NormalizeArch(f[3]), PM: f[4], Has: map[string]bool{}}
		info.Version = NormalizeVersion(f[1], f[2])
		for _, h := range strings.Split(f[5], ",") {
			if h = strings.TrimSpace(h); h != "" {
				info.Has[h] = true
			}
		}
		return info, nil
	}
	return ContainerOSInfo{}, fmt.Errorf("无法识别容器系统: %s", strings.TrimSpace(out))
}

// detectScript 探测系统/包管理器/已安装情况（checks 为逻辑包名，均已校验）
func detectScript(checks []string) string {
	var b strings.Builder
	b.WriteString(`ID=""; VERSION_ID=""
[ -r /etc/os-release ] && . /etc/os-release
[ -z "$ID" ] && [ -r /usr/lib/os-release ] && . /usr/lib/os-release
pm=""
for p in apk apt-get dnf yum zypper pacman; do if command -v $p >/dev/null 2>&1; then pm=$p; break; fi; done
has=""
if [ -x /usr/sbin/sshd ] || command -v sshd >/dev/null 2>&1; then has="$has,sshd"; fi
for f in /usr/lib/ssh/sftp-server /usr/lib/openssh/sftp-server /usr/libexec/openssh/sftp-server /usr/libexec/sftp-server /usr/lib/misc/sftp-server /usr/libexec/ssh/sftp-server /usr/lib/ssh/sftp-server; do if [ -x "$f" ]; then has="$has,sftp"; break; fi; done
pkgq() {
  case "$pm" in
    apk) apk info -e "$1" >/dev/null 2>&1 ;;
    apt-get) dpkg-query -W -f='${Status}' "$1" 2>/dev/null | grep -q "ok installed" ;;
    dnf|yum|zypper) rpm -q "$1" >/dev/null 2>&1 ;;
    pacman) pacman -Q "$1" >/dev/null 2>&1 ;;
    *) return 1 ;;
  esac
}
`)
	for _, c := range checks {
		if c == "openssh" {
			continue
		}
		fmt.Fprintf(&b, "if command -v %s >/dev/null 2>&1 || pkgq %s; then has=\"$has,%s\"; fi\n", c, c, c)
	}
	b.WriteString(`echo "LXDC|$ID|$VERSION_ID|$(uname -m)|$pm|$has"` + "\n")
	return b.String()
}

// missingPackages 根据探测结果返回尚未安装的逻辑包
func missingPackages(info ContainerOSInfo, logical []string) []string {
	var miss []string
	for _, l := range logical {
		if l == "openssh" {
			if !info.Has["sshd"] || !info.Has["sftp"] {
				miss = append(miss, l)
			}
			continue
		}
		if !info.Has[l] {
			miss = append(miss, l)
		}
	}
	return miss
}

// onlineInstallScript 在线安装（容器内 timeout 限时，避免包管理器挂住持有锁）
func onlineInstallScript(pm string, pkgs []string, timeoutSec int) string {
	list := strings.Join(pkgs, " ")
	t := strconv.Itoa(timeoutSec)
	head := `T=""; if command -v timeout >/dev/null 2>&1; then T="timeout ` + t + `"; fi` + "\n"
	switch pmFamily(pm) {
	case "apk":
		return head + `$T apk add --no-cache ` + list
	case "apt":
		return head + `export DEBIAN_FRONTEND=noninteractive
$T sh -c 'apt-get -o Acquire::Retries=0 -o Acquire::http::Timeout=15 -o Acquire::https::Timeout=15 update -qq && apt-get -o Acquire::Retries=0 install -y -qq --no-install-recommends ` + list + `'`
	case "rpm":
		return head + `if command -v dnf >/dev/null 2>&1; then $T dnf install -y -q --setopt=timeout=15 ` + list + `; else $T yum install -y -q ` + list + `; fi`
	case "zypper":
		return head + `$T zypper -n -q install ` + list
	case "pacman":
		return head + `$T pacman -Sy --noconfirm --needed ` + list
	}
	return "echo 不支持的包管理器; exit 3"
}

// pkgPushDir 容器内离线包目录
const pkgPushDir = "/tmp/lxdcloud-pkgcache"

// offlineInstallScript 用推入容器的缓存包离线安装
func offlineInstallScript(pm string, pkgs []string) string {
	D := pkgPushDir
	list := strings.Join(pkgs, " ")
	switch pmFamily(pm) {
	case "apk":
		// apk 本地仓库需 <repo>/<arch>/APKINDEX.tar.gz；有索引时只按需安装，否则直接装全部 .apk
		return `D=` + D + `; cd $D || exit 3
A=$(apk --print-arch 2>/dev/null)
if [ -n "$A" ] && [ -f APKINDEX.tar.gz ]; then
  mkdir -p "$A" && mv -f ./*.apk APKINDEX.tar.gz "$A/" 2>/dev/null
  apk add --allow-untrusted --no-cache --repositories-file /dev/null --repository "$D" ` + list + ` && exit 0
  mv -f "$A"/*.apk . 2>/dev/null
fi
apk add --allow-untrusted --no-cache ./*.apk`
	case "apt":
		// 只安装容器内尚未安装的包（缓存为完整依赖闭包，避免无谓重装 libc 等基础包）
		return `D=` + D + `; cd $D || exit 3
export DEBIAN_FRONTEND=noninteractive
need=""
for f in ./*.deb; do [ -f "$f" ] || continue; p=$(dpkg-deb -f "$f" Package 2>/dev/null); dpkg-query -W -f='${Status}' "$p" 2>/dev/null | grep -q "ok installed" || need="$need $f"; done
[ -z "$need" ] && exit 0
dpkg -i --force-confdef --force-confold $need && exit 0
dpkg -i --force-confdef --force-confold ./*.deb; dpkg --configure -a`
	case "rpm", "zypper":
		return `D=` + D + `; cd $D || exit 3
need=""
for f in ./*.rpm; do [ -f "$f" ] || continue; n=$(rpm -qp --qf '%{NAME}' "$f" 2>/dev/null); rpm -q "$n" >/dev/null 2>&1 || need="$need $f"; done
[ -z "$need" ] && exit 0
if command -v dnf >/dev/null 2>&1; then dnf install -y -q --disablerepo='*' --nogpgcheck $need && exit 0; fi
if command -v yum >/dev/null 2>&1; then yum install -y -q --disablerepo='*' --nogpgcheck $need && exit 0; fi
rpm -Uvh --nosignature $need`
	case "pacman":
		return `D=` + D + `; cd $D || exit 3
pacman -U --noconfirm --needed ./*.pkg.tar.*`
	}
	return "echo 不支持的包管理器; exit 3"
}

// sshdConfigScript 允许 root 密码登录、生成主机密钥、开机自启并启动（幂等）
const sshdConfigScript = `has_sshd() { [ -x /usr/sbin/sshd ] || command -v sshd >/dev/null 2>&1; }
has_sshd || { echo "sshd 未安装"; exit 3; }
c=/etc/ssh/sshd_config
mkdir -p /etc/ssh; touch "$c"
{ echo "PermitRootLogin yes # lxdcloud"; echo "PasswordAuthentication yes # lxdcloud"; grep -v '# lxdcloud$' "$c"; } > "$c.lxdcloud.tmp" && cat "$c.lxdcloud.tmp" > "$c"; rm -f "$c.lxdcloud.tmp"
ssh-keygen -A >/dev/null 2>&1
mkdir -p /run/sshd 2>/dev/null
if command -v rc-update >/dev/null 2>&1; then
  rc-update add sshd default >/dev/null 2>&1; rc-service sshd restart >/dev/null 2>&1 || rc-service sshd start >/dev/null 2>&1
elif command -v systemctl >/dev/null 2>&1; then
  if systemctl list-unit-files 2>/dev/null | grep -q '^ssh\.service'; then u=ssh; else u=sshd; fi
  systemctl enable "$u" >/dev/null 2>&1; systemctl restart "$u" >/dev/null 2>&1 || /usr/sbin/sshd
else
  /usr/sbin/sshd
fi
echo ok`

// ensureRunning 同一容器同一时间只跑一个安装流程（开通后紧接着改密等场景）
var ensureRunning sync.Map

// EnsureSSHServer 在容器内确保 openssh（含 sftp-server）及设置中的常用包已安装，sshd 开机自启并允许 root 密码登录。
// 先在线安装，失败/超时则使用节点本地包缓存离线安装。失败只记 Warn 日志，不影响开通。
func (c *Client) EnsureSSHServer(name string) {
	if core.GlobalConfig != nil && core.GlobalConfig.PkgCache.Disabled {
		return
	}
	settings := LoadPkgCacheSettings()
	if settings.AutoInstall != nil && !*settings.AutoInstall {
		return
	}
	if _, busy := ensureRunning.LoadOrStore(name, true); busy {
		return
	}
	defer ensureRunning.Delete(name)

	onlineTimeout := settings.OnlineTimeout
	if onlineTimeout <= 0 {
		onlineTimeout = core.PkgCacheOnlineTimeout()
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(onlineTimeout+600)*time.Second)
	defer cancel()

	logical := settings.Packages
	info, err := c.detectContainer(ctx, name, logical)
	if err != nil {
		logger.Warn("容器 %s 探测系统失败，跳过 sshd 配置: %v", name, err)
		return
	}
	miss := missingPackages(info, logical)
	if len(miss) > 0 && info.PM != "" {
		pkgs := MapPackages(info.PM, miss)
		octx, ocancel := context.WithTimeout(ctx, time.Duration(onlineTimeout+30)*time.Second)
		_, stderr, oerr := c.execOutput(octx, "exec "+name+" -- 在线安装 "+strings.Join(pkgs, " "), "exec", name, "--", "sh", "-c", onlineInstallScript(info.PM, pkgs, onlineTimeout))
		ocancel()
		if oerr != nil {
			logger.Warn("容器 %s 在线安装 %s 失败（%v %s），尝试本地包缓存", name, strings.Join(pkgs, " "), oerr, lastLine(stderr))
		}
		if info2, err := c.detectContainer(ctx, name, logical); err == nil {
			info = info2
		}
		if miss = missingPackages(info, logical); len(miss) > 0 {
			if err := c.installFromPkgCache(ctx, name, info, MapPackages(info.PM, miss)); err != nil {
				logger.Warn("容器 %s 离线安装失败: %v", name, err)
			} else if info2, err := c.detectContainer(ctx, name, logical); err == nil {
				info = info2
				miss = missingPackages(info, logical)
			}
		}
	}
	if len(miss) > 0 {
		logger.Warn("容器 %s 仍缺少: %s（系统 %s）", name, strings.Join(miss, ","), info.Key())
	}
	if !info.Has["sshd"] {
		logger.Warn("容器 %s 未能安装 sshd（无法联网且节点无 %s 的本地包缓存，可在后台「包缓存」同步）", name, info.Key())
		return
	}
	out, stderr, err := c.execOutput(ctx, "exec "+name+" -- 配置 sshd", "exec", name, "--", "sh", "-c", sshdConfigScript)
	if err != nil {
		logger.Warn("容器 %s 配置 sshd 失败: %v %s %s", name, err, strings.TrimSpace(out), lastLine(stderr))
		return
	}
	logger.Info("容器 %s sshd 已就绪", name)
}

func (c *Client) detectContainer(ctx context.Context, name string, logical []string) (ContainerOSInfo, error) {
	var lastErr error
	// 容器刚启动时 exec 可能短暂失败，重试几次
	for i := 0; i < 5; i++ {
		out, stderr, err := c.execOutput(ctx, "exec "+name+" -- 探测系统", "exec", name, "--", "sh", "-c", detectScript(logical))
		if err == nil {
			return ParseDetectOutput(out)
		}
		lastErr = fmt.Errorf("%v %s", err, lastLine(stderr))
		select {
		case <-ctx.Done():
			return ContainerOSInfo{}, lastErr
		case <-time.After(3 * time.Second):
		}
	}
	return ContainerOSInfo{}, lastErr
}

// PkgCacheEntryDir 缓存目录
func PkgCacheEntryDir(distro, version, arch string) string {
	return filepath.Join(core.PkgCacheDir(), safeSeg(distro), safeSeg(version), safeSeg(arch))
}

func isPkgFile(n string) bool {
	return strings.HasSuffix(n, ".apk") || strings.HasSuffix(n, ".deb") || strings.HasSuffix(n, ".rpm") ||
		strings.Contains(n, ".pkg.tar.") || n == "APKINDEX.tar.gz"
}

// installFromPkgCache 把本地缓存的包推入容器并离线安装
func (c *Client) installFromPkgCache(ctx context.Context, name string, info ContainerOSInfo, pkgs []string) error {
	dir := PkgCacheEntryDir(info.Distro, info.Version, info.Arch)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("节点无 %s 的本地包缓存", info.Key())
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && isPkgFile(e.Name()) {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	if len(files) == 0 {
		return fmt.Errorf("本地包缓存 %s 为空", info.Key())
	}
	if _, err := c.exec(ctx, "exec", name, "--", "sh", "-c", "rm -rf "+pkgPushDir+" && mkdir -p "+pkgPushDir); err != nil {
		return fmt.Errorf("创建容器内临时目录失败: %v", err)
	}
	// 分批推送，避免参数过长
	for i := 0; i < len(files); i += 50 {
		j := i + 50
		if j > len(files) {
			j = len(files)
		}
		args := append([]string{"file", "push"}, files[i:j]...)
		args = append(args, name+pkgPushDir+"/")
		if _, err := c.execQuiet(ctx, fmt.Sprintf("file push（%d 个缓存包）%s%s/", j-i, name, pkgPushDir), args...); err != nil {
			return fmt.Errorf("推送缓存包失败: %v", err)
		}
	}
	defer c.execOutput(context.Background(), "exec "+name+" -- 清理离线包", "exec", name, "--", "rm", "-rf", pkgPushDir)
	_, stderr, err := c.execOutput(ctx, "exec "+name+" -- 离线安装 "+strings.Join(pkgs, " "), "exec", name, "--", "sh", "-c", offlineInstallScript(info.PM, pkgs))
	if err != nil {
		return fmt.Errorf("%v %s", err, lastLine(stderr))
	}
	logger.Info("容器 %s 已用本地包缓存 %s 离线安装 %s", name, info.Key(), strings.Join(pkgs, " "))
	return nil
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "\n"); i >= 0 {
		s = s[i+1:]
	}
	if len(s) > 300 {
		s = s[:300]
	}
	return s
}

// ---------------- 同步（下载）----------------

// syncDownloadScript 在临时容器内下载完整依赖闭包到 /tmp/lxdcloud-pkgs
func syncDownloadScript(pm string, pkgs []string) string {
	list := strings.Join(pkgs, " ")
	head := "O=/tmp/lxdcloud-pkgs; rm -rf $O; mkdir -p $O; cd $O || exit 3\n"
	tail := "\nn=$(ls $O | grep -cE '\\.(apk|deb|rpm)$|\\.pkg\\.tar\\.'); echo \"LXDC-FILES $n\"; [ \"$n\" -gt 0 ]"
	switch pmFamily(pm) {
	case "apk":
		return head + `apk update -q || exit 4
apk fetch --recursive -o $O ` + list + ` || exit 5
apk index --allow-untrusted -o APKINDEX.tar.gz ./*.apk >/dev/null 2>&1 || apk index -o APKINDEX.tar.gz ./*.apk >/dev/null 2>&1 || true` + tail
	case "apt":
		return head + `export DEBIAN_FRONTEND=noninteractive
apt-get update -qq || exit 4
pk=$(apt-cache depends --recurse --no-recommends --no-suggests --no-conflicts --no-breaks --no-replaces --no-enhances ` + list + ` 2>/dev/null | grep '^[a-zA-Z0-9]' | sort -u)
[ -n "$pk" ] || exit 5
for p in $pk; do apt-get download -qq "$p" >/dev/null 2>&1 || echo "skip $p"; done` + tail
	case "rpm":
		return head + `if command -v dnf >/dev/null 2>&1; then
  dnf download --resolve --alldeps --destdir $O ` + list + ` >/dev/null 2>&1 || { dnf install -y -q dnf-plugins-core >/dev/null 2>&1; dnf download --resolve --alldeps --destdir $O ` + list + ` >/dev/null 2>&1; } || dnf install -y -q --downloadonly --downloaddir=$O ` + list + `
else
  yumdownloader --resolve --destdir $O ` + list + ` >/dev/null 2>&1 || { yum install -y -q yum-utils >/dev/null 2>&1; yumdownloader --resolve --destdir $O ` + list + ` >/dev/null 2>&1; } || yum install -y -q --downloadonly --downloaddir=$O ` + list + `
fi` + tail
	case "zypper":
		return head + `zypper -n --pkg-cache-dir $O/cache install --download-only ` + list + ` >/dev/null 2>&1
find $O/cache -name '*.rpm' -exec mv {} $O/ \; ; rm -rf $O/cache` + tail
	case "pacman":
		return head + `pacman -Syw --noconfirm --cachedir $O ` + list + ` >/dev/null 2>&1` + tail
	}
	return "echo 不支持的包管理器; exit 3"
}

// SyncPkgCacheFromImage 用指定镜像起临时容器下载包，写入本地缓存。progress 用于回报进度文字。
func (c *Client) SyncPkgCacheFromImage(ctx context.Context, image string, logical []string, progress func(string)) (*PkgCacheManifest, error) {
	if progress == nil {
		progress = func(string) {}
	}
	logical = NormalizePkgList(logical)
	name := fmt.Sprintf("lxdcloud-pkgsync-%06d", rand.Intn(1000000))
	progress("创建临时容器 " + name)
	args := []string{"init", image, name, "--ephemeral", "-c", "user.lxdcloud.temp=pkgsync", "-c", "limits.memory=512MB"}
	if core.GlobalConfig != nil && core.GlobalConfig.LXC.DefaultStorage != "" {
		args = append(args, "-s", core.GlobalConfig.LXC.DefaultStorage)
	}
	if _, err := c.exec(ctx, args...); err != nil {
		return nil, fmt.Errorf("创建临时容器失败: %v", err)
	}
	defer func() {
		dctx, dcancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer dcancel()
		c.exec(dctx, "delete", name, "--force")
	}()
	if _, err := c.exec(ctx, "start", name); err != nil {
		return nil, fmt.Errorf("启动临时容器失败: %v", err)
	}
	progress("等待临时容器网络就绪")
	deadline := time.Now().Add(90 * time.Second)
	for {
		ip, _ := c.GetContainerIP(ctx, name)
		if ip != "" {
			break
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("临时容器 90 秒内未获取到 IPv4 地址（检查 lxdbr0/DHCP）")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	info, err := c.detectContainer(ctx, name, logical)
	if err != nil {
		return nil, err
	}
	if pmFamily(info.PM) == "" {
		return nil, fmt.Errorf("镜像 %s 的包管理器不受支持（%s）", image, info.PM)
	}
	pkgs := MapPackages(info.PM, logical)
	progress(fmt.Sprintf("下载 %s 的软件包: %s", info.Key(), strings.Join(pkgs, " ")))
	out, stderr, err := c.execOutput(ctx, "exec "+name+" -- 下载软件包", "exec", name, "--", "sh", "-c", syncDownloadScript(info.PM, pkgs))
	if err != nil {
		return nil, fmt.Errorf("下载软件包失败（临时容器需能联网）: %v %s %s", err, lastLine(out), lastLine(stderr))
	}
	progress("拉取软件包到节点缓存目录")
	root := core.PkgCacheDir()
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, fmt.Errorf("创建缓存目录失败: %v", err)
	}
	tmp, err := os.MkdirTemp(root, ".pull-")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmp)
	if _, err := c.exec(ctx, "file", "pull", "-r", name+"/tmp/lxdcloud-pkgs", tmp); err != nil {
		return nil, fmt.Errorf("拉取软件包失败: %v", err)
	}
	// 收集包文件（pull -r 会在目标下建 lxdcloud-pkgs 子目录）
	staging := filepath.Join(tmp, "staging")
	os.MkdirAll(staging, 0755)
	var files int
	var size int64
	filepath.Walk(tmp, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || strings.HasPrefix(p, staging) || !isPkgFile(fi.Name()) {
			return nil
		}
		if os.Rename(p, filepath.Join(staging, fi.Name())) == nil {
			files++
			size += fi.Size()
		}
		return nil
	})
	if files == 0 {
		return nil, fmt.Errorf("未下载到任何软件包")
	}
	m := &PkgCacheManifest{Distro: info.Distro, Version: info.Version, Arch: info.Arch, PM: info.PM, Packages: logical,
		Files: files, Size: size, Image: image, SyncedAt: time.Now().Format("2006-01-02 15:04:05")}
	b, _ := json.MarshalIndent(m, "", "  ")
	os.WriteFile(filepath.Join(staging, "manifest.json"), b, 0644)
	final := PkgCacheEntryDir(info.Distro, info.Version, info.Arch)
	os.MkdirAll(filepath.Dir(final), 0755)
	old := final + ".old"
	os.RemoveAll(old)
	if _, err := os.Stat(final); err == nil {
		if err := os.Rename(final, old); err != nil {
			return nil, fmt.Errorf("替换旧缓存失败: %v", err)
		}
	}
	if err := os.Rename(staging, final); err != nil {
		os.Rename(old, final)
		return nil, fmt.Errorf("写入缓存失败: %v", err)
	}
	os.RemoveAll(old)
	progress(fmt.Sprintf("完成：%s 共 %d 个包", info.Key(), files))
	return m, nil
}

// PkgCacheEntry 缓存状态
type PkgCacheEntry struct {
	PkgCacheManifest
	Key string `json:"key"`
}

// ListPkgCache 列出缓存目录
func ListPkgCache() []PkgCacheEntry {
	root := core.PkgCacheDir()
	var out []PkgCacheEntry
	ds, _ := os.ReadDir(root)
	for _, d := range ds {
		if !d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			continue
		}
		vs, _ := os.ReadDir(filepath.Join(root, d.Name()))
		for _, v := range vs {
			if !v.IsDir() {
				continue
			}
			as, _ := os.ReadDir(filepath.Join(root, d.Name(), v.Name()))
			for _, a := range as {
				if !a.IsDir() || strings.HasSuffix(a.Name(), ".old") {
					continue
				}
				dir := filepath.Join(root, d.Name(), v.Name(), a.Name())
				e := PkgCacheEntry{Key: d.Name() + "/" + v.Name() + "/" + a.Name()}
				if b, err := os.ReadFile(filepath.Join(dir, "manifest.json")); err == nil {
					json.Unmarshal(b, &e.PkgCacheManifest)
				}
				e.Distro, e.Version, e.Arch = d.Name(), v.Name(), a.Name()
				// 以目录实际内容为准
				fs, _ := os.ReadDir(dir)
				e.Files, e.Size = 0, 0
				for _, f := range fs {
					if fi, err := f.Info(); err == nil && !f.IsDir() && isPkgFile(f.Name()) {
						e.Files++
						e.Size += fi.Size()
					}
				}
				out = append(out, e)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// DeletePkgCacheEntry 删除某个缓存目录
func DeletePkgCacheEntry(distro, version, arch string) error {
	if safeSeg(distro) != distro || safeSeg(version) != version || safeSeg(arch) != arch {
		return fmt.Errorf("参数无效")
	}
	dir := PkgCacheEntryDir(distro, version, arch)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("缓存不存在")
	}
	return os.RemoveAll(dir)
}
