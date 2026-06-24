package service

import (
	"context"
	"testing"
	"time"

	"messagefeed/internal/config"
	"messagefeed/internal/llm"
	"messagefeed/internal/notifier"
)

func TestAdminConfigServiceStatusMasksSecretsAndBuildsEndpoints(t *testing.T) {
	service := NewAdminConfigService(testAdminConfig(), WithAdminConfigWeChatWorkCallbackConfigured(true), WithAdminConfigNow(fixedAdminNow))

	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.WeChatWork.CorpIDMasked == "ww0123456789abcdef" {
		t.Fatal("CorpIDMasked exposed raw corp id")
	}
	if status.WeChatWork.CallbackURL != "https://example.test/api/v1/channels/wechat-work/app/callback" {
		t.Fatalf("CallbackURL = %q", status.WeChatWork.CallbackURL)
	}
	if status.WeChatWork.OAuthCallbackURL != "https://example.test/api/v1/auth/wechat-work/callback" {
		t.Fatalf("OAuthCallbackURL = %q", status.WeChatWork.OAuthCallbackURL)
	}
	if !status.WeChatWork.Enabled || !status.WeChatWork.OAuthConfigured || !status.WeChatWork.CallbackConfigured {
		t.Fatalf("wechat work status = %#v", status.WeChatWork)
	}
	if !status.LLM.Enabled || status.LLM.APIKeyPresent != true {
		t.Fatalf("llm status = %#v", status.LLM)
	}
	if status.Requirements[0].Items[2].Key != "WECHAT_WORK_SECRET" || !status.Requirements[0].Items[2].Secret {
		t.Fatalf("secret requirement metadata = %#v", status.Requirements[0].Items[2])
	}
	if status.UpdatedAt != fixedAdminNow().UTC() {
		t.Fatalf("UpdatedAt = %s", status.UpdatedAt)
	}
}

func TestAdminConfigServiceTestLLM(t *testing.T) {
	llmClient := &fakeAdminConfigLLM{
		response: llm.ChatResponse{Provider: "hyb", Model: "model-a", Content: "OK"},
	}
	service := NewAdminConfigService(testAdminConfig(), WithAdminConfigLLM(llmClient), WithAdminConfigNow(fixedAdminNow))

	result, err := service.TestLLM(context.Background(), AdminLLMTestInput{})
	if err != nil {
		t.Fatalf("TestLLM() error = %v", err)
	}
	if result.Status != "succeeded" || result.Provider != "hyb" || result.Model != "model-a" {
		t.Fatalf("result = %#v", result)
	}
	if llmClient.calls != 1 {
		t.Fatalf("llm calls = %d", llmClient.calls)
	}
	if len(llmClient.lastRequest.Messages) != 2 {
		t.Fatalf("messages = %#v", llmClient.lastRequest.Messages)
	}
}

func TestAdminConfigServiceTestLLMRequiresConfiguredClient(t *testing.T) {
	service := NewAdminConfigService(testAdminConfig())

	if _, err := service.TestLLM(context.Background(), AdminLLMTestInput{}); err == nil {
		t.Fatal("TestLLM() error = nil, want error")
	}
}

func TestAdminConfigServiceTestWeChatWork(t *testing.T) {
	sender := &fakeAdminConfigWeChatWorkSender{result: notifier.WeChatWorkSendResult{ErrCode: 0, MessageID: "wx-msg-1"}}
	service := NewAdminConfigService(testAdminConfig(), WithAdminConfigWeChatWorkSender(sender), WithAdminConfigNow(fixedAdminNow))

	result, err := service.TestWeChatWork(context.Background(), AdminWeChatWorkTestInput{ToUser: "zhangsan"})
	if err != nil {
		t.Fatalf("TestWeChatWork() error = %v", err)
	}
	if result.Status != "succeeded" || result.MessageID != "wx-msg-1" {
		t.Fatalf("result = %#v", result)
	}
	if sender.lastMessage.ToUser != "zhangsan" {
		t.Fatalf("ToUser = %q", sender.lastMessage.ToUser)
	}
	if sender.lastMessage.Content == "" {
		t.Fatal("default content is empty")
	}
}

func TestAdminConfigServiceTestWeChatWorkRequiresRecipient(t *testing.T) {
	service := NewAdminConfigService(testAdminConfig(), WithAdminConfigWeChatWorkSender(&fakeAdminConfigWeChatWorkSender{}))

	if _, err := service.TestWeChatWork(context.Background(), AdminWeChatWorkTestInput{}); err == nil {
		t.Fatal("TestWeChatWork() error = nil, want error")
	}
}

func testAdminConfig() config.Config {
	cfg := config.Defaults()
	cfg.Runtime.PublicBaseURL = "https://example.test"
	cfg.Database.DSN = "postgres://messagefeed:password@localhost:5432/messagefeed?sslmode=disable"
	cfg.WeChatWork = config.WeChatWorkConfig{
		CorpID:         "ww0123456789abcdef",
		AgentID:        "1000002",
		Secret:         "wechat-secret",
		CallbackToken:  "callback-token",
		EncodingAESKey: "abcdefghijklmnopqrstuvwxyzABCDEFG1234567890",
	}
	cfg.LLM = config.LLMConfig{
		Provider: "hyb",
		APIKey:   "llm-key",
		BaseURL:  "https://llm.example/v1",
		Model:    "model-a",
	}
	return cfg
}

func fixedAdminNow() time.Time {
	return time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
}

type fakeAdminConfigLLM struct {
	calls       int
	lastRequest llm.ChatRequest
	response    llm.ChatResponse
	err         error
}

func (f *fakeAdminConfigLLM) Chat(_ context.Context, request llm.ChatRequest) (llm.ChatResponse, error) {
	f.calls++
	f.lastRequest = request
	if f.err != nil {
		return llm.ChatResponse{}, f.err
	}
	return f.response, nil
}

type fakeAdminConfigWeChatWorkSender struct {
	lastMessage notifier.WeChatWorkTextMessage
	result      notifier.WeChatWorkSendResult
	err         error
}

func (f *fakeAdminConfigWeChatWorkSender) SendText(_ context.Context, message notifier.WeChatWorkTextMessage) (notifier.WeChatWorkSendResult, error) {
	f.lastMessage = message
	if f.err != nil {
		return notifier.WeChatWorkSendResult{}, f.err
	}
	if f.result.ErrMsg == "" && f.result.ErrCode == 0 {
		f.result.ErrMsg = "ok"
	}
	return f.result, nil
}
