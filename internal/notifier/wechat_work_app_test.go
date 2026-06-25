package notifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWeChatWorkAppClientSendTextGetsTokenAndSendsMessage(t *testing.T) {
	tokenCalls := 0
	sendCalls := 0
	var sent sendTextRequest
	var sentAccessToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/gettoken":
			tokenCalls++
			if r.URL.Query().Get("corpid") != "corp-a" {
				t.Fatalf("corpid = %q", r.URL.Query().Get("corpid"))
			}
			if r.URL.Query().Get("corpsecret") != "secret-a" {
				t.Fatalf("corpsecret = %q", r.URL.Query().Get("corpsecret"))
			}
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","access_token":"token-a","expires_in":7200}`))
		case "/cgi-bin/message/send":
			sendCalls++
			sentAccessToken = r.URL.Query().Get("access_token")
			if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
				t.Fatalf("decode send request: %v", err)
			}
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","msgid":"msg-1"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewWeChatWorkAppClient(WeChatWorkAppConfig{
		CorpID:     "corp-a",
		AgentID:    "1000002",
		Secret:     "secret-a",
		APIBaseURL: server.URL,
		Now:        func() time.Time { return time.Date(2026, 6, 24, 16, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("NewWeChatWorkAppClient() error = %v", err)
	}

	result, err := client.SendText(context.Background(), WeChatWorkTextMessage{
		ToUser:  "zhangsan",
		Content: "AI 回复",
	})
	if err != nil {
		t.Fatalf("SendText() error = %v", err)
	}

	if tokenCalls != 1 {
		t.Fatalf("tokenCalls = %d, want 1", tokenCalls)
	}
	if sendCalls != 1 {
		t.Fatalf("sendCalls = %d, want 1", sendCalls)
	}
	if sentAccessToken != "token-a" {
		t.Fatalf("access token = %q", sentAccessToken)
	}
	if sent.ToUser != "zhangsan" {
		t.Fatalf("touser = %q", sent.ToUser)
	}
	if sent.AgentID != "1000002" {
		t.Fatalf("agentid = %q", sent.AgentID)
	}
	if sent.Text.Content != "AI 回复" {
		t.Fatalf("content = %q", sent.Text.Content)
	}
	if result.MessageID != "msg-1" {
		t.Fatalf("MessageID = %q", result.MessageID)
	}
}

func TestWeChatWorkAppClientSendTemplateCard(t *testing.T) {
	var sent sendTemplateCardRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/gettoken":
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","access_token":"token-a","expires_in":7200}`))
		case "/cgi-bin/message/send":
			if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
				t.Fatalf("decode send request: %v", err)
			}
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","msgid":"msg-card-1"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewWeChatWorkAppClient(WeChatWorkAppConfig{
		CorpID:     "corp-a",
		AgentID:    "1000002",
		Secret:     "secret-a",
		APIBaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewWeChatWorkAppClient() error = %v", err)
	}
	result, err := client.SendTemplateCard(context.Background(), WeChatWorkTemplateCardMessage{
		ToUser:      "zhangsan",
		Title:       "Agent 实时工作进度",
		Description: "执行中",
		URL:         "https://example.test/agent/plans/1",
		Buttons: []WeChatWorkTemplateCardButton{
			{Key: "view_progress", Text: "查看进度", URL: "https://example.test/agent/plans/1"},
		},
	})
	if err != nil {
		t.Fatalf("SendTemplateCard() error = %v", err)
	}
	if result.MessageID != "msg-card-1" {
		t.Fatalf("MessageID = %q", result.MessageID)
	}
	if sent.MsgType != "template_card" || sent.TemplateCard.CardType != "text_notice" {
		t.Fatalf("sent = %#v", sent)
	}
	if sent.TemplateCard.MainTitle.Title != "Agent 实时工作进度" || sent.TemplateCard.CardAction.URL == "" || len(sent.TemplateCard.JumpList) != 1 {
		t.Fatalf("template card = %#v", sent.TemplateCard)
	}
}

func TestWeChatWorkAppClientCachesAccessToken(t *testing.T) {
	tokenCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/gettoken":
			tokenCalls++
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","access_token":"token-a","expires_in":7200}`))
		case "/cgi-bin/message/send":
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","msgid":"msg-1"}`))
		}
	}))
	defer server.Close()

	client, err := NewWeChatWorkAppClient(WeChatWorkAppConfig{
		CorpID:     "corp-a",
		AgentID:    "1000002",
		Secret:     "secret-a",
		APIBaseURL: server.URL,
		Now:        func() time.Time { return time.Date(2026, 6, 24, 16, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("NewWeChatWorkAppClient() error = %v", err)
	}

	for i := 0; i < 2; i++ {
		if _, err := client.SendText(context.Background(), WeChatWorkTextMessage{ToUser: "zhangsan", Content: "AI 回复"}); err != nil {
			t.Fatalf("SendText() error = %v", err)
		}
	}
	if tokenCalls != 1 {
		t.Fatalf("tokenCalls = %d, want 1", tokenCalls)
	}
}

func TestWeChatWorkAppClientTruncatesTextByBytes(t *testing.T) {
	var sent sendTextRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/gettoken":
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","access_token":"token-a","expires_in":7200}`))
		case "/cgi-bin/message/send":
			if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
				t.Fatalf("decode send request: %v", err)
			}
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","msgid":"msg-1"}`))
		}
	}))
	defer server.Close()

	client, err := NewWeChatWorkAppClient(WeChatWorkAppConfig{
		CorpID:     "corp-a",
		AgentID:    "1000002",
		Secret:     "secret-a",
		APIBaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewWeChatWorkAppClient() error = %v", err)
	}

	if _, err := client.SendText(context.Background(), WeChatWorkTextMessage{
		ToUser:  "zhangsan",
		Content: strings.Repeat("你", 800),
	}); err != nil {
		t.Fatalf("SendText() error = %v", err)
	}

	if len([]byte(sent.Text.Content)) > WeChatWorkTextByteLimit {
		t.Fatalf("sent content bytes = %d, limit %d", len([]byte(sent.Text.Content)), WeChatWorkTextByteLimit)
	}
}

func TestWeChatWorkAppClientReturnsProviderErrorDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/gettoken":
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","access_token":"token-a","expires_in":7200}`))
		case "/cgi-bin/message/send":
			_, _ = w.Write([]byte(`{"errcode":81013,"errmsg":"invalid user","invaliduser":"lisi"}`))
		}
	}))
	defer server.Close()

	client, err := NewWeChatWorkAppClient(WeChatWorkAppConfig{
		CorpID:     "corp-a",
		AgentID:    "1000002",
		Secret:     "secret-a",
		APIBaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewWeChatWorkAppClient() error = %v", err)
	}

	result, err := client.SendText(context.Background(), WeChatWorkTextMessage{ToUser: "lisi", Content: "AI 回复"})
	if err == nil {
		t.Fatal("SendText() error = nil, want provider error")
	}
	if result.InvalidUser != "lisi" {
		t.Fatalf("InvalidUser = %q, want lisi", result.InvalidUser)
	}
}
