package lxc

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// fakeApk 模拟 Alpine 仓库：openssh 依赖闭包 + install_if 的 *-openrc 子包
const fakeApk = `#!/bin/sh
echo "$*" >> "$FAKE_APK_LOG"
REPO="openssh-server-9.9_p2-r0 openssh-server-common-9.9_p2-r0 openssh-sftp-server-9.9_p2-r0 openssh-keygen-9.9_p2-r0 libcrypto3-3.3.2-r1 musl-1.2.5-r0 zlib-1.3.1-r1 curl-8.11.1-r0 openssh-server-common-openrc-9.9_p2-r0 openrc-0.55.1-r2 musl-openrc-1.2.5-r0"
case "$1" in
  update|index) [ "$1" = index ] && : > APKINDEX.tar.gz; exit 0 ;;
  --print-arch) echo x86_64; exit 0 ;;
  info) [ "$2" = -e ] || exit 1; case " $FAKE_INSTALLED " in *" $3 "*) exit 0 ;; esac; exit 1 ;;
  fetch)
    shift; o=.; names=""
    while [ $# -gt 0 ]; do case "$1" in -o) o=$2; shift 2 ;; -*) shift ;; *) names="$names $1"; shift ;; esac; done
    for n in $names; do
      hit=""
      for p in $REPO; do b=$(echo "$p" | sed -E 's/-[0-9][^-]*-r[0-9]+$//'); [ "$b" = "$n" ] && hit=$p; done
      [ -n "$hit" ] || { echo "ERROR: unable to select packages: $n" >&2; exit 1; }
      : > "$o/$hit.apk"
      case "$n" in openssh-server) for d in openssh-server-common-9.9_p2-r0 openssh-keygen-9.9_p2-r0 libcrypto3-3.3.2-r1 musl-1.2.5-r0 zlib-1.3.1-r1; do : > "$o/$d.apk"; done ;; esac
    done; exit 0 ;;
  add) exit 0 ;;
esac
exit 0
`

func setupFakeApk(t *testing.T) (bin, log string) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("无 sh")
	}
	for _, c := range []string{"sed", "sort", "grep", "ls", "basename"} {
		if _, err := exec.LookPath(c); err != nil {
			t.Skip("缺少 " + c)
		}
	}
	d := t.TempDir()
	bin = filepath.Join(d, "bin")
	os.MkdirAll(bin, 0755)
	if err := os.WriteFile(filepath.Join(bin, "apk"), []byte(fakeApk), 0755); err != nil {
		t.Fatal(err)
	}
	return bin, filepath.Join(d, "apk.log")
}

func runScript(t *testing.T, bin, log, script string, env ...string) string {
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = append(os.Environ(), append([]string{"PATH=" + bin + ":" + os.Getenv("PATH"), "FAKE_APK_LOG=" + log}, env...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("脚本失败: %v\n%s", err, out)
	}
	return string(out)
}

// 同步时必须把依赖闭包中各包的 -openrc 子包一起下载（openssh-server-common-openrc 提供 /etc/init.d/sshd）
func TestApkSyncFetchesOpenRCSubpackages(t *testing.T) {
	bin, log := setupFakeApk(t)
	o := filepath.Join(t.TempDir(), "pkgs")
	out := runScript(t, bin, log, syncDownloadScriptIn("apk", MapPackages("apk", []string{"openssh", "curl"}), o))
	es, _ := os.ReadDir(o)
	var names []string
	for _, e := range es {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	got := strings.Join(names, " ")
	for _, want := range []string{"openssh-server-common-openrc-9.9_p2-r0.apk", "musl-openrc-1.2.5-r0.apk", "openssh-server-9.9_p2-r0.apk", "APKINDEX.tar.gz"} {
		if !strings.Contains(got, want) {
			t.Errorf("缓存缺少 %s：%s", want, got)
		}
	}
	if !strings.Contains(out, "LXDC-OPENRC openssh-server-common-openrc") || !strings.Contains(out, "LXDC-FILES") {
		t.Errorf("输出: %s", out)
	}
	// 不存在 openrc 子包的不应导致失败，也不应尝试 -openrc-openrc
	b, _ := os.ReadFile(log)
	if strings.Contains(string(b), "openrc-openrc") {
		t.Errorf("不应对 openrc 子包再找子包: %s", b)
	}
}

// 离线安装后补装「主包已安装」的 openrc 子包，且不装主包未安装的
func TestApkOfflineInstallsOpenRC(t *testing.T) {
	bin, log := setupFakeApk(t)
	d := t.TempDir()
	for _, f := range []string{"openssh-server-9.9_p2-r0.apk", "openssh-server-common-9.9_p2-r0.apk", "openssh-server-common-openrc-9.9_p2-r0.apk",
		"musl-openrc-1.2.5-r0.apk", "foo-openrc-1.0-r0.apk", "APKINDEX.tar.gz"} {
		os.WriteFile(filepath.Join(d, f), nil, 0644)
	}
	runScript(t, bin, log, offlineInstallScriptIn("apk", []string{"openssh-server", "openssh-sftp-server"}, d),
		"FAKE_INSTALLED=openssh-server openssh-server-common musl openrc musl-openrc")
	b, _ := os.ReadFile(log)
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	last := lines[len(lines)-1]
	if !strings.HasPrefix(last, "add ") || !strings.Contains(last, "openssh-server-common-openrc") ||
		strings.Contains(last, "foo-openrc") || strings.Contains(last, "musl-openrc") {
		t.Fatalf("补装 openrc 子包的命令不对: %s\n全部: %s", last, b)
	}
	if _, err := os.Stat(filepath.Join(d, "x86_64", "openssh-server-common-openrc-9.9_p2-r0.apk")); err != nil {
		t.Fatal("包应移入 <arch>/ 本地仓库目录")
	}
	// A:noarch 的包 apk 会到 <repo>/noarch/ 下找（七牛实测 openssh-server-common 因此报 "package mentioned in index not found"）
	if _, err := os.Stat(filepath.Join(d, "noarch", "openssh-server-common-openrc-9.9_p2-r0.apk")); err != nil {
		t.Fatal("noarch 应指向架构目录")
	}
	if s := offlineInstallScript("apk", []string{"x"}); !strings.Contains(s, "--repositories-file /dev/null ./*.apk") {
		t.Fatal("兜底分支必须禁用远程仓库，避免离线时卡住")
	}
}

func TestSshdInitCheck(t *testing.T) {
	if !sshdInitMissing("LXDC-INITD missing\n") || sshdInitMissing("LXDC-INITD ok") || sshdInitMissing("LXDC-INITD n/a") || sshdInitMissing("") {
		t.Fatal("sshdInitMissing 判断错误")
	}
	cmd := exec.Command("sh", "-n")
	cmd.Stdin = strings.NewReader(sshdInitCheckScript)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("语法错误: %v %s", err, out)
	}
}
