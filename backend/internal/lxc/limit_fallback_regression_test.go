package lxc

import "testing"

func TestIsLimitUnsupportedErrorRealCases(t *testing.T) {
	cases := []struct{ msg string; want bool }{
		// 盒子真实报错（myserver 2026-10-20 日志）
		{"启动容器失败: Error: Failed to start device \"eth0\": Failed to create root tc qdisc: Failed to run: tc qdisc add dev veth27b2bb6c handle 1:0 root htb default 10: exit status 2 (Error: Specified qdisc kind is unknown.)", true},
		{"Error: Failed to start device \"eth0\": Failed to create tc filter: exit status 2 (Error: TC classifier not found)", true},
		{"RTNETLINK answers: Operation not supported", true},
		{"Invalid value for device option \"limits.ingress\": unsupported", true},
		{"some unrelated error: connection refused", false},
	}
	for _, c := range cases {
		got := IsLimitUnsupportedError(errMsg(c.msg))
		if got != c.want {
			t.Errorf("msg=%q got %v want %v", c.msg, got, c.want)
		}
	}
}

type errMsg string
func (e errMsg) Error() string { return string(e) }
