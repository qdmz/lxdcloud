package service

import (
	"bytes"
	"io"
	"net/http"
	"testing"
)

// 转发时已去掉 Accept-Encoding（722d584），节点返回明文 JSON：令牌应被登记且响应体不变
func TestRecordConsoleTokenPlain(t *testing.T) {
	body := []byte(`{"code":200,"data":{"token":"tok-plain-1"}}`)
	resp := &http.Response{Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(body))}
	recordConsoleToken(resp, 3)
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, body) {
		t.Fatalf("响应体被改动")
	}
	consoleTokenNodes.Lock()
	e, ok := consoleTokenNodes.m["tok-plain-1"]
	consoleTokenNodes.Unlock()
	if !ok || e.nodeID != 3 {
		t.Fatalf("令牌未登记")
	}
}

func TestRecordConsoleTokenFailedResponse(t *testing.T) {
	body := []byte(`{"code":500,"msg":"x"}`)
	resp := &http.Response{Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(body))}
	recordConsoleToken(resp, 3)
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, body) {
		t.Fatalf("响应体被改动")
	}
}
