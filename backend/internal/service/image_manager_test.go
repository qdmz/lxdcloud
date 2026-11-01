package service

import (
	"encoding/json"
	"testing"
)

const ssFixture = `{"products":{
 "alpine:3.22:amd64:default":{"aliases":"alpine/3.22/default,alpine/3.22","arch":"amd64","os":"Alpine","release":"3.22","release_title":"3.22","variant":"default",
  "versions":{"20261015_13:00":{"items":{"lxd.tar.xz":{"ftype":"lxd.tar.xz","size":900,"path":"images/a/old/lxd.tar.xz","combined_squashfs_sha256":"1111111111111111111111111111111111111111111111111111111111111111"},"root.squashfs":{"ftype":"squashfs","size":3000,"path":"images/a/old/rootfs.squashfs"}}},
   "20261016_13:00":{"items":{"lxd.tar.xz":{"ftype":"lxd.tar.xz","size":912,"path":"images/a/new/lxd.tar.xz","sha256":"aa","combined_squashfs_sha256":"016b0caf6422687535e3a62ab54d8c224160f39d968c61419bee7953e61e4d78"},"root.squashfs":{"ftype":"squashfs","size":3401324,"path":"images/a/new/rootfs.squashfs"}}}}},
 "debian:12:arm64:default":{"aliases":"debian/12","arch":"arm64","os":"Debian","release":"bookworm","release_title":"12","variant":"default",
  "versions":{"20261016":{"items":{"incus.tar.xz":{"ftype":"incus.tar.xz","size":800,"path":"images/d/incus.tar.xz","sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","combined_squashfs_sha256":"2222222222222222222222222222222222222222222222222222222222222222"},"root.squashfs":{"ftype":"squashfs","size":5000,"path":"images/d/root.squashfs","sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}}}}},
 "vmonly:1:amd64:default":{"arch":"amd64","os":"VM","release":"1","versions":{"1":{"items":{"disk.qcow2":{"ftype":"disk-kvm.img","size":1}}}}},
 "com.ubuntu.cloud:server:24.04:amd64":{"aliases":"24.04,n,noble","arch":"amd64","os":"ubuntu","release":"noble","release_title":"24.04 LTS","version":"24.04",
  "versions":{"20261017":{"items":{"lxd.tar.xz":{"ftype":"lxd.tar.xz","size":412,"path":"server/noble/lxd.tar.xz","combined_sha256":"35002ee5ed4f5d8011599ed68697cecdb4dd8bf1dccecdd1f3e58a0816c8ccb9","combined_squashfs_sha256":"12a0de07e1683f41b169af0612897eae462efdeb84322281e91724c97e32fe0d"},"squashfs":{"ftype":"squashfs","size":276336640,"path":"server/noble/x.squashfs"}}}}}
}}`

func TestParseSimplestreams(t *testing.T) {
	var doc struct {
		Products map[string]ssProduct `json:"products"`
	}
	if err := json.Unmarshal([]byte(ssFixture), &doc); err != nil {
		t.Fatal(err)
	}
	items := ParseSimplestreamsProducts(doc.Products)
	if len(items) != 3 {
		t.Fatalf("应解析出 3 个容器镜像，实际 %d", len(items))
	}
	m := map[string]RemoteImage{}
	for _, it := range items {
		m[it.Key] = it
	}
	a := m["alpine:3.22:amd64:default"]
	if a.Method != "lxd" || a.Version != "20261016_13:00" || a.Fingerprint != "016b0caf6422687535e3a62ab54d8c224160f39d968c61419bee7953e61e4d78" || len(a.Aliases) != 2 || a.Size != 912+3401324 {
		t.Fatalf("alpine 解析错误: %+v", a)
	}
	d := m["debian:12:arm64:default"]
	if d.Method != "files" || d.MetaPath != "images/d/incus.tar.xz" || d.RootfsPath != "images/d/root.squashfs" || d.Arch != "arm64" || d.RootfsSHA256 == "" {
		t.Fatalf("incus 格式应走文件导入: %+v", d)
	}
	u := m["com.ubuntu.cloud:server:24.04:amd64"]
	if u.Fingerprint != "12a0de07e1683f41b169af0612897eae462efdeb84322281e91724c97e32fe0d" || u.Method != "lxd" || u.ReleaseTitle != "24.04 LTS" {
		t.Fatalf("ubuntu 解析错误: %+v", u)
	}
}

func TestDownloadRequestValidate(t *testing.T) {
	ok := ImageDownloadRequest{Server: "https://images.lxd.canonical.com/", Protocol: "simplestreams", Fingerprint: "016b0caf6422687535e3a62ab54d8c224160f39d968c61419bee7953e61e4d78", Method: "lxd", Alias: "alpine-3.22"}
	if err := ok.Validate(); err != nil || ok.Server != "https://images.lxd.canonical.com" || ok.Label != "alpine-3.22" {
		t.Fatalf("合法请求被拒: %v %+v", err, ok)
	}
	bad := []ImageDownloadRequest{
		{Server: "ftp://x", Protocol: "simplestreams", Fingerprint: "016b0caf6422"},
		{Server: "https://x", Protocol: "oci", Fingerprint: "016b0caf6422"},
		{Server: "http://x", Protocol: "lxd", Fingerprint: "016b0caf6422"},
		{Server: "https://x", Protocol: "simplestreams"},
		{Server: "https://x", Protocol: "simplestreams", Fingerprint: "zz;rm"},
		{Server: "https://x", Protocol: "simplestreams", Fingerprint: "016b0caf6422", Alias: "-bad"},
		{Server: "https://x", Protocol: "simplestreams", Fingerprint: "016b0caf6422", Alias: "a/../b"},
		{Server: "https://x", Protocol: "simplestreams", Fingerprint: "016b0caf6422", AutoUpdate: true},
		{Server: "https://x", Protocol: "simplestreams", Method: "files", MetaPath: "../etc/passwd", RootfsPath: "a"},
		{Server: "https://x", Protocol: "simplestreams", Method: "files", MetaPath: "/abs", RootfsPath: "a"},
		{Server: "https://x", Protocol: "simplestreams", Method: "wget", Fingerprint: "016b0caf6422"},
		{Server: "https://u:p@x", Protocol: "simplestreams", Fingerprint: "016b0caf6422"},
	}
	for i, r := range bad {
		if err := r.Validate(); err == nil {
			t.Errorf("第 %d 个非法请求未被拒绝: %+v", i, r)
		}
	}
	f := ImageDownloadRequest{Server: "https://mirrors.tuna.tsinghua.edu.cn/lxc-images", Protocol: "simplestreams", Method: "files", MetaPath: "images/debian/12/amd64/default/20261016_05:24/incus.tar.xz", RootfsPath: "images/debian/12/amd64/default/20261016_05:24/rootfs.squashfs", AutoUpdate: true}
	if err := f.Validate(); err != nil || f.AutoUpdate {
		t.Fatalf("文件方式校验错误: %v", err)
	}
}

func TestImportURLValidate(t *testing.T) {
	r := ImageImportURLRequest{URL: "https://example.com/a.tar.gz", Alias: "debian-12"}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, b := range []ImageImportURLRequest{{URL: "file:///etc/passwd", Alias: "a"}, {URL: "https://e/a", Alias: ""}, {URL: "https://e/a", RootfsURL: "gopher://x", Alias: "a"}} {
		if err := b.Validate(); err == nil {
			t.Errorf("非法导入未被拒绝: %+v", b)
		}
	}
	if !ValidImageAlias("ubuntu/24.04") || ValidImageAlias("x;y") || ValidImageAlias("a/") || ValidFingerprint("0123") || !ValidFingerprint("0123456789ab") {
		t.Error("别名/指纹校验错误")
	}
}
