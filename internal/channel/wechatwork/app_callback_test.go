package wechatwork

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
)

const (
	testCorpID  = "ww0123456789abcdef"
	testAgentID = "1000002"
	testToken   = "test-token"
)

func TestVerifyURLDecryptsEchoString(t *testing.T) {
	codec := newTestCodec(t)
	encrypted := encryptForTest(t, codec, []byte("callback-ok"))
	input := VerifyURLInput{
		MsgSignature: Signature(testToken, "1780000000", "nonce-a", encrypted),
		Timestamp:    "1780000000",
		Nonce:        "nonce-a",
		EchoStr:      encrypted,
	}

	got, err := codec.VerifyURL(input)
	if err != nil {
		t.Fatalf("VerifyURL() error = %v", err)
	}
	if got != "callback-ok" {
		t.Fatalf("VerifyURL() = %q, want callback-ok", got)
	}
}

func TestParseInboundMessageStandardizesTextMessage(t *testing.T) {
	codec := newTestCodec(t)
	plainXML := []byte(`<xml><ToUserName><![CDATA[` + testCorpID + `]]></ToUserName><FromUserName><![CDATA[zhangsan]]></FromUserName><CreateTime>1780000001</CreateTime><MsgType><![CDATA[text]]></MsgType><Content><![CDATA[最近有什么更新]]></Content><MsgId>987654321</MsgId><AgentID>` + testAgentID + `</AgentID></xml>`)
	encrypted := encryptForTest(t, codec, plainXML)
	body := []byte(fmt.Sprintf(`<xml><ToUserName><![CDATA[%s]]></ToUserName><AgentID>%s</AgentID><Encrypt><![CDATA[%s]]></Encrypt></xml>`, testCorpID, testAgentID, encrypted))

	message, err := codec.ParseInboundMessage(ParseInboundMessageInput{
		MsgSignature: Signature(testToken, "1780000001", "nonce-b", encrypted),
		Timestamp:    "1780000001",
		Nonce:        "nonce-b",
		Body:         body,
	})
	if err != nil {
		t.Fatalf("ParseInboundMessage() error = %v", err)
	}

	if message.Provider != ProviderWeChatWorkApp {
		t.Fatalf("Provider = %q, want %q", message.Provider, ProviderWeChatWorkApp)
	}
	if message.ProviderMessageID != "987654321" {
		t.Fatalf("ProviderMessageID = %q, want 987654321", message.ProviderMessageID)
	}
	if message.DedupeKey != ProviderWeChatWorkApp+":987654321" {
		t.Fatalf("DedupeKey = %q", message.DedupeKey)
	}
	if message.ExternalUserID != "zhangsan" {
		t.Fatalf("ExternalUserID = %q, want zhangsan", message.ExternalUserID)
	}
	if message.MsgType != "text" {
		t.Fatalf("MsgType = %q, want text", message.MsgType)
	}
	if message.TextContent != "最近有什么更新" {
		t.Fatalf("TextContent = %q", message.TextContent)
	}
	if message.CreateTimeUnix != 1780000001 {
		t.Fatalf("CreateTimeUnix = %d", message.CreateTimeUnix)
	}
	if !strings.Contains(message.RawXML, "<MsgType><![CDATA[text]]></MsgType>") {
		t.Fatalf("RawXML does not contain plaintext message: %q", message.RawXML)
	}
}

func TestParseInboundMessageRejectsInvalidSignature(t *testing.T) {
	codec := newTestCodec(t)
	plainXML := []byte(`<xml><FromUserName><![CDATA[zhangsan]]></FromUserName><MsgType><![CDATA[text]]></MsgType><Content><![CDATA[test]]></Content><MsgId>1</MsgId><AgentID>` + testAgentID + `</AgentID></xml>`)
	encrypted := encryptForTest(t, codec, plainXML)
	body := []byte(fmt.Sprintf(`<xml><AgentID>%s</AgentID><Encrypt><![CDATA[%s]]></Encrypt></xml>`, testAgentID, encrypted))

	if _, err := codec.ParseInboundMessage(ParseInboundMessageInput{
		MsgSignature: "invalid",
		Timestamp:    "1780000001",
		Nonce:        "nonce-b",
		Body:         body,
	}); err == nil {
		t.Fatal("ParseInboundMessage() error = nil, want invalid signature error")
	}
}

func newTestCodec(t *testing.T) *AppCallbackCodec {
	t.Helper()
	codec, err := NewAppCallbackCodec(AppCallbackConfig{
		CorpID:         testCorpID,
		AgentID:        testAgentID,
		CallbackToken:  testToken,
		EncodingAESKey: testEncodingAESKey(),
	})
	if err != nil {
		t.Fatalf("NewAppCallbackCodec() error = %v", err)
	}
	return codec
}

func testEncodingAESKey() string {
	key := []byte("0123456789abcdefghijklmnopqrstuv")
	return strings.TrimRight(base64.StdEncoding.EncodeToString(key), "=")
}

func encryptForTest(t *testing.T, codec *AppCallbackCodec, message []byte) string {
	t.Helper()
	random := []byte("abcdefghijklmnop")
	payload := make([]byte, 0, len(random)+4+len(message)+len(testCorpID)+32)
	payload = append(payload, random...)
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(message)))
	payload = append(payload, size[:]...)
	payload = append(payload, message...)
	payload = append(payload, []byte(testCorpID)...)
	payload = pkcs7PadForTest(payload, 32)

	block, err := aes.NewCipher(codec.aesKey)
	if err != nil {
		t.Fatalf("aes.NewCipher() error = %v", err)
	}
	ciphertext := make([]byte, len(payload))
	cipher.NewCBCEncrypter(block, codec.aesKey[:aes.BlockSize]).CryptBlocks(ciphertext, payload)
	return base64.StdEncoding.EncodeToString(ciphertext)
}

func pkcs7PadForTest(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	if padding == 0 {
		padding = blockSize
	}
	return append(data, bytes.Repeat([]byte{byte(padding)}, padding)...)
}
