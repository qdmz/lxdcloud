package opengfw

import (
	"os"

	"fmt"
	"lxdapi/pkg/logger"
	"strings"
	"testing"
)

// 实际在七牛节点上看到的格式（nft 1.0.9 把 "queue num 100 bypass" 显示为 "queue flags bypass to 100"）
const listingDup = `table inet lxdfilter {
	chain forward { # handle 1
		type filter hook forward priority filter; policy accept;
		iifname "lxdbr0" queue flags bypass to 100 # handle 4
		oifname "lxdbr0" queue flags bypass to 100 # handle 5
		iifname "lxdbr0" queue flags bypass to 100 # handle 6
		oifname "lxdbr0" queue flags bypass to 100 # handle 7
		ip saddr 10.0.0.5 drop # handle 8
		iifname "lxdbr0" queue flags bypass to 101 # handle 9
		iifname "eth0" queue num 100 # handle 10
	}
}`

func TestParseQueueRulesFormats(t *testing.T) {
	cases := map[string]string{
		`iifname "lxdbr0" queue num 100 bypass # handle 3`:                                             "in",
		`iifname "lxdbr0" queue to 100 bypass # handle 3`:                                              "in",
		`oifname "lxdbr0" queue flags bypass to 100 # handle 3`:                                        "out",
		`oifname "lxdbr0" counter packets 5 bytes 300 queue flags bypass,fanout to 100-101 # handle 3`: "out",
		`iifname "lxdbr0" queue flags bypass to 100 comment "lxdcloud-opengfw-q100-in" # handle 3`:     "in",
		`meta l4proto tcp queue flags bypass to 100 comment "lxdcloud-opengfw-q100-out" # handle 3`:    "out",
		`iifname "lxdbr0" queue flags bypass to 1000 # handle 3`:                                       "",
		`iifname "lxdbr0" queue num 10 # handle 3`:                                                     "",
		`iifname "lxdbr0" accept # handle 3`:                                                           "",
		`iifname "lxdbr0" queue num 100 bypass`:                                                        "", // 无 handle（非 -a 输出）
	}
	for line, want := range cases {
		rs := parseQueueRules("\t\t"+line, 100)
		got := ""
		if len(rs) == 1 {
			got = rs[0].Dir
			if rs[0].Handle != "3" {
				t.Errorf("%q handle=%s", line, rs[0].Handle)
			}
		} else if len(rs) > 1 {
			got = "multi"
		}
		if got != want {
			t.Errorf("%q: got %q want %q", line, got, want)
		}
	}
	rs := parseQueueRules(listingDup, 100)
	if len(rs) != 4 {
		t.Fatalf("应识别 4 条重复规则，得到 %+v", rs)
	}
}

func TestPlanDedup(t *testing.T) {
	keep, del := planDedup(parseQueueRules(listingDup, 100))
	if keep["in"].Handle != "4" || keep["out"].Handle != "5" || strings.Join(del, ",") != "6,7" {
		t.Fatalf("keep=%+v del=%v", keep, del)
	}
	// 带标记的优先保留
	l := `iifname "lxdbr0" queue num 100 # handle 2
iifname "lxdbr0" queue flags bypass to 100 comment "lxdcloud-opengfw-q100-in" # handle 3
iifname "lxdbr0" queue num 100 # handle 4`
	keep, del = planDedup(parseQueueRules(l, 100))
	if keep["in"].Handle != "3" || strings.Join(del, ",") != "2,4" || len(keep) != 1 {
		t.Fatalf("keep=%+v del=%v", keep, del)
	}
}

// fakeNFT 模拟 lxdfilter forward 链，按 nft 1.0.9 的显示格式输出
type fakeNFT struct {
	rules      map[int]string
	order      []int
	next       int
	calls      []string
	noComment  bool
	failAddOut bool
}

func newFake(lines ...string) *fakeNFT {
	f := &fakeNFT{rules: map[int]string{}, next: 2}
	for _, l := range lines {
		f.rules[f.next] = l
		f.order = append(f.order, f.next)
		f.next++
	}
	return f
}

func (f *fakeNFT) run(args ...string) ([]byte, error) {
	j := strings.Join(args, " ")
	f.calls = append(f.calls, j)
	switch {
	case strings.HasPrefix(j, "-a list chain inet lxdfilter forward"):
		var b strings.Builder
		b.WriteString("table inet lxdfilter {\n\tchain forward { # handle 1\n\t\ttype filter hook forward priority filter; policy accept;\n")
		for _, h := range f.order {
			fmt.Fprintf(&b, "\t\t%s # handle %d\n", f.rules[h], h)
		}
		b.WriteString("\t}\n}\n")
		return []byte(b.String()), nil
	case strings.HasPrefix(j, "add rule inet lxdfilter forward "):
		if f.noComment && strings.Contains(j, "comment") {
			return []byte("Error: syntax error, unexpected comment"), fmt.Errorf("exit 1")
		}
		if f.failAddOut && strings.Contains(j, "oifname") {
			return []byte("Error: boom"), fmt.Errorf("exit 1")
		}
		rest := strings.TrimPrefix(j, "add rule inet lxdfilter forward ")
		rest = strings.Replace(rest, "queue num 100 bypass", "queue flags bypass to 100", 1)
		f.rules[f.next] = rest
		f.order = append(f.order, f.next)
		f.next++
		return nil, nil
	case strings.HasPrefix(j, "delete rule inet lxdfilter forward handle "):
		var h int
		fmt.Sscanf(strings.TrimPrefix(j, "delete rule inet lxdfilter forward handle "), "%d", &h)
		if _, ok := f.rules[h]; !ok {
			return []byte("Error: no such rule"), fmt.Errorf("exit 1")
		}
		delete(f.rules, h)
		for i, x := range f.order {
			if x == h {
				f.order = append(f.order[:i], f.order[i+1:]...)
				break
			}
		}
		return nil, nil
	}
	return nil, nil
}

func (f *fakeNFT) count() (in, out int) {
	l, _ := f.run("-a", "list", "chain", "inet", "lxdfilter", "forward")
	for _, r := range parseQueueRules(string(l), 100) {
		if r.Dir == "in" {
			in++
		} else {
			out++
		}
	}
	return
}

func withFake(t *testing.T, f *fakeNFT) {
	oldRun, oldSave := nftRun, persistSave
	nftRun, persistSave = f.run, func() {}
	t.Cleanup(func() { nftRun, persistSave = oldRun, oldSave })
}

func TestSetupIdempotentAndDedup(t *testing.T) {
	// 历史上累积的 10 对重复规则 + 一条无关规则
	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, `iifname "lxdbr0" queue flags bypass to 100`, `oifname "lxdbr0" queue flags bypass to 100`)
	}
	lines = append(lines, `ip saddr 10.0.0.5 drop`)
	f := newFake(lines...)
	withFake(t, f)
	m := NewNFTablesManager(100)
	for i := 0; i < 5; i++ { // 多次启动
		if err := m.Setup(); err != nil {
			t.Fatal(err)
		}
		if in, out := f.count(); in != 1 || out != 1 {
			t.Fatalf("第 %d 次 Setup 后 in=%d out=%d", i+1, in, out)
		}
	}
	if len(f.rules) != 3 {
		t.Fatalf("无关规则不应被删除: %v", f.rules)
	}
	for _, c := range f.calls {
		if strings.HasPrefix(c, "add rule") {
			t.Fatalf("已有规则时不应再添加: %s", c)
		}
	}
	if err := m.Remove(); err != nil {
		t.Fatal(err)
	}
	if in, out := f.count(); in+out != 0 || len(f.rules) != 1 {
		t.Fatalf("Remove 后应只剩无关规则: %v", f.rules)
	}
}

func TestSetupFreshAddsTaggedPair(t *testing.T) {
	f := newFake()
	withFake(t, f)
	m := NewNFTablesManager(100)
	for i := 0; i < 3; i++ {
		if err := m.Setup(); err != nil {
			t.Fatal(err)
		}
	}
	if in, out := f.count(); in != 1 || out != 1 {
		t.Fatalf("in=%d out=%d rules=%v", in, out, f.rules)
	}
	for _, r := range f.rules {
		if !strings.Contains(r, `comment "lxdcloud-opengfw-q100-`) {
			t.Fatalf("新规则应带 comment 标记: %s", r)
		}
	}
	// 只缺一个方向时只补该方向
	for h, r := range f.rules {
		if strings.HasPrefix(r, "oifname") {
			f.run("delete", "rule", "inet", "lxdfilter", "forward", "handle", fmt.Sprint(h))
		}
	}
	if err := m.Setup(); err != nil {
		t.Fatal(err)
	}
	if in, out := f.count(); in != 1 || out != 1 {
		t.Fatalf("补齐后 in=%d out=%d", in, out)
	}
}

func TestSetupFallbackAndRollback(t *testing.T) {
	f := newFake()
	f.noComment = true
	withFake(t, f)
	m := NewNFTablesManager(100)
	if err := m.Setup(); err != nil {
		t.Fatal(err)
	}
	m.Setup()
	if in, out := f.count(); in != 1 || out != 1 {
		t.Fatalf("不支持 comment 时 in=%d out=%d", in, out)
	}
	g := newFake()
	g.failAddOut = true
	withFake(t, g)
	if err := m.Setup(); err == nil {
		t.Fatal("添加失败应返回错误")
	}
	if in, out := g.count(); in+out != 0 {
		t.Fatalf("失败后应回滚本次新增: %v", g.rules)
	}
}

func TestMain(m *testing.M) {
	logger.Init("release", false)
	os.Exit(m.Run())
}
