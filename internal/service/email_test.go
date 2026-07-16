package service

import "testing"

func TestExtractCodes(t *testing.T) {
	inbox := []MailItem{
		{
			Subject: "FB3-PDG xAI confirmation code",
			Body:    `<h1>Validate</h1><td style="font-size:26px; font-weight:bold">FB3-PDG</td>`,
		},
		{
			Subject: "你的 OpenAI 临时验证码",
			Body:    `输入此临时验证码以继续： <p>697871</p>`,
		},
	}
	codes := ExtractCodes(inbox, nil)
	if len(codes) == 0 {
		t.Fatal("expected codes")
	}
	found := map[string]bool{}
	for _, c := range codes {
		found[c] = true
	}
	if !found["FB3-PDG"] {
		t.Fatalf("missing FB3-PDG in %v", codes)
	}
	if !found["697871"] {
		t.Fatalf("missing 697871 in %v", codes)
	}
}
