package handler

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"messagefeed/internal/channel/wechatwork"
)

func TestWeChatWorkCallbackRoutesRequireConfiguredCodec(t *testing.T) {
	router := newTestRouter(t, RouterOptions{})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/channels/wechat-work/app/callback", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestWeChatWorkVerifyURLReturnsPlainText(t *testing.T) {
	callback := &fakeWeChatWorkAppCallback{verifyEcho: "callback-ok"}
	router := newTestRouter(t, RouterOptions{WeChatWorkAppCallback: callback})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/channels/wechat-work/app/callback?msg_signature=sig&timestamp=ts&nonce=nonce&echostr=echo", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Body.String(); got != "callback-ok" {
		t.Fatalf("body = %q, want callback-ok", got)
	}
	if callback.verifyInput.MsgSignature != "sig" {
		t.Fatalf("MsgSignature = %q, want sig", callback.verifyInput.MsgSignature)
	}
	if callback.verifyInput.EchoStr != "echo" {
		t.Fatalf("EchoStr = %q, want echo", callback.verifyInput.EchoStr)
	}
}

func TestWeChatWorkPostCallbackReturnsSuccess(t *testing.T) {
	callback := &fakeWeChatWorkAppCallback{}
	router := newTestRouter(t, RouterOptions{WeChatWorkAppCallback: callback})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/channels/wechat-work/app/callback?msg_signature=sig&timestamp=ts&nonce=nonce", strings.NewReader("<xml></xml>"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Body.String(); got != "success" {
		t.Fatalf("body = %q, want success", got)
	}
	if callback.parseInput.MsgSignature != "sig" {
		t.Fatalf("MsgSignature = %q, want sig", callback.parseInput.MsgSignature)
	}
	if string(callback.parseInput.Body) != "<xml></xml>" {
		t.Fatalf("Body = %q", string(callback.parseInput.Body))
	}
}

func TestWeChatWorkPostCallbackRejectsOversizedBody(t *testing.T) {
	callback := &fakeWeChatWorkAppCallback{}
	router := newTestRouter(t, RouterOptions{WeChatWorkAppCallback: callback})
	body := io.LimitReader(strings.NewReader(strings.Repeat("x", int(maxWeChatWorkCallbackBodyBytes)+1)), maxWeChatWorkCallbackBodyBytes+1)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/channels/wechat-work/app/callback?msg_signature=sig&timestamp=ts&nonce=nonce", body)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusRequestEntityTooLarge)
	}
	if callback.parseInput.Body != nil {
		t.Fatal("ParseInboundMessage was called for oversized body")
	}
}

type fakeWeChatWorkAppCallback struct {
	verifyInput wechatwork.VerifyURLInput
	parseInput  wechatwork.ParseInboundMessageInput
	verifyEcho  string
	verifyErr   error
	parseErr    error
}

func (f *fakeWeChatWorkAppCallback) VerifyURL(input wechatwork.VerifyURLInput) (string, error) {
	f.verifyInput = input
	if f.verifyErr != nil {
		return "", f.verifyErr
	}
	return f.verifyEcho, nil
}

func (f *fakeWeChatWorkAppCallback) ParseInboundMessage(input wechatwork.ParseInboundMessageInput) (wechatwork.InboundMessage, error) {
	f.parseInput = input
	if f.parseErr != nil {
		return wechatwork.InboundMessage{}, f.parseErr
	}
	if input.Body == nil {
		return wechatwork.InboundMessage{}, errors.New("missing body")
	}
	return wechatwork.InboundMessage{Provider: wechatwork.ProviderWeChatWorkApp}, nil
}
