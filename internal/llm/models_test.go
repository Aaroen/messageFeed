package llm

import (
	"strings"
	"testing"
)

func TestDecodeModelCatalog(test *testing.T) {
	models, err := decodeModelCatalog(strings.NewReader(`{"object":"list","data":[{"id":"group/deepseek-flash","owned_by":"group","reasoning":true}],"has_more":false}`))
	if err != nil || len(models) != 1 || models[0].ID != "group/deepseek-flash" || models[0].OwnedBy != "group" {
		test.Fatalf("通用模型目录解析失败：%v", err)
	}
	for _, input := range []string{`{}`, `{"data":null}`, `{"data":[{"id":""}]}`, `{"data":[{"id":" model"}]}`, `{"data":[{"id":"same"},{"id":"same"}]}`, `{"data":[]}invalid`, strings.Repeat(" ", (4<<20)+1)} {
		if _, err := decodeModelCatalog(strings.NewReader(input)); err == nil {
			test.Fatal("无效模型目录没有被拒绝")
		}
	}
	if models, err := decodeModelCatalog(strings.NewReader(`{"data":[]}`)); err != nil || len(models) != 0 {
		test.Fatal("有效空目录解析失败")
	}
}
