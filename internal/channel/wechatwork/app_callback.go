package wechatwork

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"

	"messagefeed/internal/domain"
)

const (
	ProviderWeChatWorkApp = "wechat_work_app"
	directChatType        = "direct"
)

// AppCallbackConfig 保存企业微信自建应用回调协议所需配置。
type AppCallbackConfig struct {
	CorpID         string
	AgentID        string
	CallbackToken  string
	EncodingAESKey string
}

// AppCallbackCodec 负责企业微信自建应用回调验签、解密和标准化。
type AppCallbackCodec struct {
	corpID        string
	agentID       string
	callbackToken string
	aesKey        []byte
}

type VerifyURLInput struct {
	MsgSignature string
	Timestamp    string
	Nonce        string
	EchoStr      string
}

type ParseInboundMessageInput struct {
	MsgSignature string
	Timestamp    string
	Nonce        string
	Body         []byte
}

type InboundMessage struct {
	Provider          string
	ProviderMessageID string
	DedupeKey         string
	CorpID            string
	AgentID           string
	ExternalUserID    string
	ChatID            string
	ChatType          string
	MsgType           string
	TextContent       string
	EventType         string
	EventKey          string
	CreateTimeUnix    int64
	RawXML            string
}

type encryptedEnvelope struct {
	ToUserName string `xml:"ToUserName"`
	AgentID    string `xml:"AgentID"`
	Encrypt    string `xml:"Encrypt"`
}

type plainMessage struct {
	ToUserName   string `xml:"ToUserName"`
	FromUserName string `xml:"FromUserName"`
	CreateTime   int64  `xml:"CreateTime"`
	MsgType      string `xml:"MsgType"`
	Content      string `xml:"Content"`
	MsgID        string `xml:"MsgId"`
	AgentID      string `xml:"AgentID"`
	Event        string `xml:"Event"`
	EventKey     string `xml:"EventKey"`
}

func NewAppCallbackCodec(cfg AppCallbackConfig) (*AppCallbackCodec, error) {
	cfg.CorpID = strings.TrimSpace(cfg.CorpID)
	cfg.AgentID = strings.TrimSpace(cfg.AgentID)
	cfg.CallbackToken = strings.TrimSpace(cfg.CallbackToken)
	cfg.EncodingAESKey = strings.TrimSpace(cfg.EncodingAESKey)
	if cfg.CorpID == "" {
		return nil, invalidInput("missing corp id", nil)
	}
	if cfg.AgentID == "" {
		return nil, invalidInput("missing agent id", nil)
	}
	if cfg.CallbackToken == "" {
		return nil, invalidInput("missing callback token", nil)
	}
	if len(cfg.EncodingAESKey) != 43 {
		return nil, invalidInput("encoding aes key must be 43 characters", nil)
	}

	aesKey, err := base64.StdEncoding.DecodeString(cfg.EncodingAESKey + "=")
	if err != nil {
		return nil, invalidInput("invalid encoding aes key", err)
	}
	if len(aesKey) != 32 {
		return nil, invalidInput("encoding aes key must decode to 32 bytes", nil)
	}

	return &AppCallbackCodec{
		corpID:        cfg.CorpID,
		agentID:       cfg.AgentID,
		callbackToken: cfg.CallbackToken,
		aesKey:        aesKey,
	}, nil
}

func (c *AppCallbackCodec) VerifyURL(input VerifyURLInput) (string, error) {
	if c == nil {
		return "", unavailable("wechat work callback codec unavailable", nil)
	}
	encrypted := strings.TrimSpace(input.EchoStr)
	if err := c.validateSignature(input.MsgSignature, input.Timestamp, input.Nonce, encrypted); err != nil {
		return "", err
	}
	plaintext, err := c.decrypt(encrypted)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func (c *AppCallbackCodec) ParseInboundMessage(input ParseInboundMessageInput) (InboundMessage, error) {
	if c == nil {
		return InboundMessage{}, unavailable("wechat work callback codec unavailable", nil)
	}
	if len(input.Body) == 0 {
		return InboundMessage{}, invalidInput("empty callback body", nil)
	}

	var envelope encryptedEnvelope
	if err := xml.Unmarshal(input.Body, &envelope); err != nil {
		return InboundMessage{}, invalidInput("invalid callback xml", err)
	}
	encrypted := strings.TrimSpace(envelope.Encrypt)
	if encrypted == "" {
		return InboundMessage{}, invalidInput("missing encrypted payload", nil)
	}
	if envelope.AgentID != "" && c.agentID != "" && envelope.AgentID != c.agentID {
		return InboundMessage{}, invalidInput("callback agent id mismatch", nil)
	}
	if err := c.validateSignature(input.MsgSignature, input.Timestamp, input.Nonce, encrypted); err != nil {
		return InboundMessage{}, err
	}

	plaintext, err := c.decrypt(encrypted)
	if err != nil {
		return InboundMessage{}, err
	}

	var message plainMessage
	if err := xml.Unmarshal(plaintext, &message); err != nil {
		return InboundMessage{}, invalidInput("invalid decrypted callback xml", err)
	}
	agentID := strings.TrimSpace(message.AgentID)
	if agentID == "" {
		agentID = strings.TrimSpace(envelope.AgentID)
	}
	if c.agentID != "" && agentID != "" && agentID != c.agentID {
		return InboundMessage{}, invalidInput("message agent id mismatch", nil)
	}

	providerMessageID := strings.TrimSpace(message.MsgID)
	if providerMessageID == "" {
		providerMessageID = fallbackProviderMessageID(input, plaintext)
	}

	return InboundMessage{
		Provider:          ProviderWeChatWorkApp,
		ProviderMessageID: providerMessageID,
		DedupeKey:         ProviderWeChatWorkApp + ":" + providerMessageID,
		CorpID:            c.corpID,
		AgentID:           agentID,
		ExternalUserID:    strings.TrimSpace(message.FromUserName),
		ChatID:            strings.TrimSpace(message.FromUserName),
		ChatType:          directChatType,
		MsgType:           strings.TrimSpace(message.MsgType),
		TextContent:       strings.TrimSpace(message.Content),
		EventType:         strings.TrimSpace(message.Event),
		EventKey:          strings.TrimSpace(message.EventKey),
		CreateTimeUnix:    message.CreateTime,
		RawXML:            string(plaintext),
	}, nil
}

func (c *AppCallbackCodec) validateSignature(signature string, timestamp string, nonce string, encrypted string) error {
	signature = strings.TrimSpace(signature)
	if signature == "" || timestamp == "" || nonce == "" || encrypted == "" {
		return invalidInput("missing callback signature parameters", nil)
	}
	expected := Signature(c.callbackToken, timestamp, nonce, encrypted)
	if !strings.EqualFold(signature, expected) {
		return invalidInput("invalid callback signature", nil)
	}
	return nil
}

func (c *AppCallbackCodec) decrypt(encrypted string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, invalidInput("invalid encrypted payload encoding", err)
	}
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, invalidInput("invalid encrypted payload size", nil)
	}

	block, err := aes.NewCipher(c.aesKey)
	if err != nil {
		return nil, invalidInput("invalid aes key", err)
	}
	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, c.aesKey[:aes.BlockSize]).CryptBlocks(plaintext, ciphertext)

	unpadded, err := pkcs7Unpad(plaintext)
	if err != nil {
		return nil, invalidInput("invalid encrypted payload padding", err)
	}
	if len(unpadded) < 20 {
		return nil, invalidInput("invalid encrypted payload layout", nil)
	}
	messageLength := int(binary.BigEndian.Uint32(unpadded[16:20]))
	messageStart := 20
	messageEnd := messageStart + messageLength
	if messageLength < 0 || messageEnd > len(unpadded) {
		return nil, invalidInput("invalid encrypted payload message length", nil)
	}
	receiveID := string(unpadded[messageEnd:])
	if receiveID != c.corpID {
		return nil, invalidInput("callback corp id mismatch", nil)
	}
	return unpadded[messageStart:messageEnd], nil
}

// Signature 计算企业微信回调签名，供生产代码和协议测试复用。
func Signature(token string, timestamp string, nonce string, encrypted string) string {
	parts := []string{token, timestamp, nonce, encrypted}
	sort.Strings(parts)
	h := sha1.New()
	_, _ = h.Write([]byte(strings.Join(parts, "")))
	return hex.EncodeToString(h.Sum(nil))
}

func fallbackProviderMessageID(input ParseInboundMessageInput, plaintext []byte) string {
	h := sha256.New()
	_, _ = h.Write([]byte(ProviderWeChatWorkApp))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(input.MsgSignature))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(input.Timestamp))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(input.Nonce))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(plaintext)
	return hex.EncodeToString(h.Sum(nil))
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}
	padding := int(data[len(data)-1])
	if padding < 1 || padding > 32 || padding > len(data) {
		return nil, fmt.Errorf("invalid padding size")
	}
	if !bytes.Equal(data[len(data)-padding:], bytes.Repeat([]byte{byte(padding)}, padding)) {
		return nil, fmt.Errorf("invalid padding bytes")
	}
	return data[:len(data)-padding], nil
}

func invalidInput(message string, err error) error {
	return domain.NewAppError(domain.ErrorKindInvalidInput, "wechat_work_invalid_input", message, "wechatwork.callback", false, err)
}

func unavailable(message string, err error) error {
	return domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_unavailable", message, "wechatwork.callback", true, err)
}
