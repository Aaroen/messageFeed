package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"messagefeed/internal/channel/wechatwork"
	"messagefeed/internal/service"
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

func TestWeChatWorkPostCallbackPassesMessageToReceiver(t *testing.T) {
	callback := &fakeWeChatWorkAppCallback{
		parseMessage: wechatwork.InboundMessage{
			Provider:          wechatwork.ProviderWeChatWorkApp,
			ProviderMessageID: "msg-1",
			CorpID:            "corp-a",
			AgentID:           "1000002",
			ExternalUserID:    "zhangsan",
			ChatID:            "zhangsan",
			ChatType:          "direct",
			MsgType:           "text",
			TextContent:       "最近有什么更新",
			RawXML:            "<xml></xml>",
		},
	}
	receiver := &fakeWeChatWorkInboundReceiver{}
	router := newTestRouter(t, RouterOptions{
		WeChatWorkAppCallback: callback,
		WeChatWorkReceiver:    receiver,
	})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/channels/wechat-work/app/callback?msg_signature=sig&timestamp=ts&nonce=nonce", strings.NewReader("<xml></xml>"))
	request.Header.Set(requestIDHeader, "request-1")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}
	if receiver.input.ProviderMessageID != "msg-1" {
		t.Fatalf("ProviderMessageID = %q", receiver.input.ProviderMessageID)
	}
	if receiver.input.TextContent != "最近有什么更新" {
		t.Fatalf("TextContent = %q", receiver.input.TextContent)
	}
	if receiver.input.RequestID != "request-1" {
		t.Fatalf("RequestID = %q", receiver.input.RequestID)
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
	verifyInput  wechatwork.VerifyURLInput
	parseInput   wechatwork.ParseInboundMessageInput
	verifyEcho   string
	verifyErr    error
	parseErr     error
	parseMessage wechatwork.InboundMessage
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
	if f.parseMessage.Provider != "" {
		return f.parseMessage, nil
	}
	return wechatwork.InboundMessage{Provider: wechatwork.ProviderWeChatWorkApp}, nil
}

type fakeWeChatWorkInboundReceiver struct {
	input service.ReceiveWeChatWorkAppMessageInput
	err   error
}

func (f *fakeWeChatWorkInboundReceiver) ReceiveWeChatWorkAppMessage(_ context.Context, input service.ReceiveWeChatWorkAppMessageInput) (service.ReceiveWeChatWorkAppMessageResult, error) {
	f.input = input
	if f.err != nil {
		return service.ReceiveWeChatWorkAppMessageResult{}, f.err
	}
	return service.ReceiveWeChatWorkAppMessageResult{}, nil
}
