package lxc

import (
	"os/exec"
	"strings"
	"testing"
)

func TestParseDetectOutput(t *testing.T) {
	info, err := ParseDetectOutput("noise\nLXDC|alpine|3.22.2|x86_64|apk|,sshd,wget\n")
	if err != nil {
		t.Fatal(err)
	}
	if info.Key() != "alpine/3.22/amd64" || info.PM != "apk" || !info.Has["sshd"] || info.Has["sftp"] || !info.Has["wget"] {
		t.Fatalf("解析错误: %+v", info)
	}
	if m := missingPackages(info, []string{"openssh", "wget", "curl"}); strings.Join(m, ",") != "openssh,curl" {
		t.Fatalf("缺失包计算错误: %v", m)
	}
	if _, err := ParseDetectOutput("garbage"); err == nil {
		t.Fatal("应报错")
	}
}

func TestNormalizeVersionArch(t *testing.T) {
	cases := [][3]string{{"debian", "12", "12"}, {"ubuntu", "24.04", "24.04"}, {"rocky", "9.4", "9"}, {"alpine", "3.20.3", "3.20"},
		{"arch", "", "rolling"}, {"opensuse-tumbleweed", "20241010", "rolling"}, {"opensuse-leap", "15.6", "15.6"}, {"debian", "", "rolling"}}
	for _, c := range cases {
		if got := NormalizeVersion(c[0], c[1]); got != c[2] {
			t.Errorf("NormalizeVersion(%s,%s)=%s want %s", c[0], c[1], got, c[2])
		}
	}
	if NormalizeArch("aarch64") != "arm64" || NormalizeArch("x86_64") != "amd64" || NormalizeArch("../x") != "x" {
		t.Error("NormalizeArch 错误")
	}
}

func TestMapPackagesAndNormalize(t *testing.T) {
	if got := strings.Join(MapPackages("apk", []string{"openssh", "curl"}), " "); got != "openssh-server openssh-sftp-server openssh-keygen curl" {
		t.Fatalf("apk: %s", got)
	}
	if got := strings.Join(MapPackages("apt-get", []string{"openssh", "wget"}), " "); got != "openssh-server openssh-sftp-server wget" {
		t.Fatalf("apt: %s", got)
	}
	if got := strings.Join(MapPackages("dnf", []string{"openssh"}), " "); got != "openssh-server" {
		t.Fatalf("dnf: %s", got)
	}
	if got := strings.Join(NormalizePkgList(nil), ","); got != "openssh,wget,curl" {
		t.Fatalf("默认列表: %s", got)
	}
	if got := strings.Join(NormalizePkgList([]string{"curl", "sshd", "rm -rf /", "Sudo", "curl"}), ","); got != "openssh,curl,sudo" {
		t.Fatalf("规范化: %s", got)
	}
}

// 生成的脚本必须是合法的 POSIX sh
func TestScriptsSyntax(t *testing.T) {
	pk := []string{"openssh-server", "curl"}
	scripts := map[string]string{"detect": detectScript([]string{"openssh", "wget", "curl"}), "sshd": sshdConfigScript}
	for _, pm := range []string{"apk", "apt-get", "dnf", "yum", "zypper", "pacman", "unknown"} {
		scripts["online-"+pm] = onlineInstallScript(pm, pk, 90)
		scripts["offline-"+pm] = offlineInstallScript(pm, pk)
		scripts["sync-"+pm] = syncDownloadScript(pm, pk)
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("无 sh")
	}
	for n, s := range scripts {
		cmd := exec.Command(sh, "-n")
		cmd.Stdin = strings.NewReader(s)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("%s 脚本语法错误: %v %s", n, err, out)
		}
	}
	if !strings.Contains(scripts["sync-apk"], "apk fetch --recursive") || !strings.Contains(scripts["offline-apk"], "--allow-untrusted") ||
		!strings.Contains(scripts["offline-apt-get"], "dpkg -i") || !strings.Contains(scripts["offline-dnf"], "rpm -Uvh") ||
		!strings.Contains(scripts["sync-dnf"], "--resolve") || !strings.Contains(scripts["sync-apt-get"], "apt-get download") {
		t.Error("脚本缺少关键命令")
	}
	if !strings.Contains(sshdConfigScript, "PermitRootLogin yes") || !strings.Contains(sshdConfigScript, "PasswordAuthentication yes") || !strings.Contains(sshdConfigScript, "ssh-keygen -A") {
		t.Error("sshd 配置脚本不完整")
	}
}
