package llm

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"messagefeed/internal/domain"
)

type OutputSchema struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
	Strict bool            `json:"strict"`
}

func SchemaFor[T any](name string, required ...string) *OutputSchema {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		panic(err)
	}
	schema.Required = required
	definition, err := json.Marshal(schema)
	if err != nil {
		panic(err)
	}
	return &OutputSchema{Name: name, Schema: definition}
}

func (request ChatRequest) nativeSchema() *OutputSchema {
	if request.SchemaLocalOnly {
		return nil
	}
	return request.OutputSchema
}

type chatOutputFormat struct {
	Type   string        `json:"type"`
	Schema *OutputSchema `json:"json_schema"`
}

type responsesOutputFormat struct {
	Format struct {
		Type string `json:"type"`
		*OutputSchema
	} `json:"format"`
}

func chatSchemaFormat(schema *OutputSchema) *chatOutputFormat {
	if schema == nil {
		return nil
	}
	return &chatOutputFormat{Type: "json_schema", Schema: schema}
}

func responsesSchemaFormat(schema *OutputSchema) *responsesOutputFormat {
	if schema == nil {
		return nil
	}
	format := &responsesOutputFormat{}
	format.Format.Type, format.Format.OutputSchema = "json_schema", schema
	return format
}

func (schema *OutputSchema) Validate(content string) (string, error) {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") && strings.HasSuffix(content, "```") {
		content = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(content, "```"), "json"), "```"))
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(content), &value); err != nil {
		code := "structured_output_invalid"
		if errors.Is(err, io.ErrUnexpectedEOF) || strings.Contains(err.Error(), "unexpected end") {
			code = "structured_output_incomplete"
		}
		return "", structuredOutputError(code, err)
	}
	if value == nil {
		return "", structuredOutputError("structured_output_invalid", nil)
	}
	var definition jsonschema.Schema
	if err := json.Unmarshal(schema.Schema, &definition); err != nil {
		return "", err
	}
	resolved, err := definition.Resolve(nil)
	if err != nil {
		return "", err
	}
	if err := resolved.Validate(value); err != nil {
		return "", structuredOutputError("structured_output_invalid", err)
	}
	return content, nil
}

func structuredOutputError(code string, cause error) error {
	return domain.NewAppError(domain.ErrorKindUnavailable, code, "模型未提供有效的结构化决策", "llm.structured_output", true, cause)
}

func CanRetryStructuredOutput(err error) bool {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case "structured_output_invalid", "structured_output_incomplete", "llm_response_incomplete":
			return true
		}
	}
	var routeErr *llmRouteError
	if !errors.As(err, &routeErr) || routeErr.statusCode != 400 && routeErr.statusCode != 422 {
		return false
	}
	message := strings.ToLower(routeErr.Error())
	return strings.Contains(message, "response_format") || strings.Contains(message, "json_schema") || strings.Contains(message, "text.format")
}
