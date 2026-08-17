//go:build unit || !integration

package models

import (
	"bytes"
	"html/template"
	"testing"

	null "gopkg.in/volatiletech/null.v6"
)

func TestCampaign_CompileTemplate_Exhaustive(t *testing.T) {
	fMap := template.FuncMap{
		"TrackLink": func(url string, args ...any) string { return url },
		"TrackView": func(args ...any) template.HTML { return template.HTML("") },
	}

	// 1. Standard HTML campaign compilation
	campHTML := Campaign{
		Subject:      "Welcome {{ .Name }}",
		TemplateType: TemplateTypeCampaign,
		TemplateBody: "<html><body>{{ template \"content\" . }}</body></html>",
		ContentType:  CampaignContentTypeHTML,
		Body:         "<h1>Hello {{ .Name }}</h1>",
		AltBody:      null.StringFrom("Hello {{ .Name }}"),
		Headers: Headers{
			map[string]string{"X-Custom-Header": "Header-{{ .Name }}"},
		},
	}

	if err := campHTML.CompileTemplate(fMap); err != nil {
		t.Fatalf("unexpected error compiling HTML campaign: %v", err)
	}

	if campHTML.Tpl == nil {
		t.Fatalf("expected compiled Tpl for HTML campaign")
	}
	if campHTML.SubjectTpl == nil {
		t.Fatalf("expected compiled SubjectTpl")
	}
	if campHTML.AltBodyTpl == nil {
		t.Fatalf("expected compiled AltBodyTpl")
	}
	if len(campHTML.HeaderTpls) != 1 || campHTML.HeaderTpls[0]["X-Custom-Header"] == nil {
		t.Fatalf("expected compiled HeaderTpls")
	}

	// Execute compiled template
	var buf bytes.Buffer
	if err := campHTML.Tpl.ExecuteTemplate(&buf, BaseTpl, map[string]string{"Name": "Alice"}); err != nil {
		t.Fatalf("unexpected error executing compiled template: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Hello Alice")) {
		t.Fatalf("rendered body missing expected content, got: %s", buf.String())
	}

	// 2. Markdown campaign compilation
	campMD := Campaign{
		Subject:      "Markdown Notice",
		TemplateType: TemplateTypeCampaign,
		TemplateBody: "<html><body>{{ template \"content\" . }}</body></html>",
		ContentType:  CampaignContentTypeMarkdown,
		Body:         "# Hello {{ .Name }}\n* Welcome to our service.",
	}
	if err := campMD.CompileTemplate(fMap); err != nil {
		t.Fatalf("unexpected error compiling Markdown campaign: %v", err)
	}

	// 3. Prompt Campaign compilation with System Prompt
	campPrompt := Campaign{
		Subject:      "Prompt Subject {{ .Name }}",
		TemplateType: TemplateTypePrompt,
		TemplateBody: "System prompt for {{ .Name }}",
		ContentType:  CampaignContentTypeHTML,
		Body:         "User message {{ .Name }}",
	}
	if err := campPrompt.CompileTemplate(fMap); err != nil {
		t.Fatalf("unexpected error compiling Prompt campaign: %v", err)
	}
	if campPrompt.SystemPromptTpl == nil {
		t.Fatalf("expected SystemPromptTpl to be compiled")
	}
}

func TestCampaign_CompileTemplate_ErrorBranches(t *testing.T) {
	fMap := template.FuncMap{}

	// Bad Subject
	badSubj := Campaign{
		Subject:      "Bad {{ .Name | invalidFunc }}",
		TemplateBody: "<html></html>",
		Body:         "Body",
	}
	if err := badSubj.CompileTemplate(fMap); err == nil {
		t.Fatalf("expected error compiling bad subject expression")
	}

	// Bad System Prompt
	badSysPrompt := Campaign{
		TemplateType: TemplateTypePrompt,
		TemplateBody: "System {{ .Name | invalidFunc }}",
		Body:         "Body",
	}
	if err := badSysPrompt.CompileTemplate(fMap); err == nil {
		t.Fatalf("expected error compiling bad system prompt expression")
	}

	// Bad Base Template
	badBase := Campaign{
		TemplateBody: "<html>{{ .Name | invalidFunc }}</html>",
		Body:         "Body",
	}
	if err := badBase.CompileTemplate(fMap); err == nil {
		t.Fatalf("expected error compiling bad base template")
	}

	// Bad Body Template
	badBody := Campaign{
		TemplateBody: "<html>{{ template \"content\" . }}</html>",
		Body:         "Body {{ .Name | invalidFunc }}",
	}
	if err := badBody.CompileTemplate(fMap); err == nil {
		t.Fatalf("expected error compiling bad body template")
	}

	// Bad Alt Body
	badAlt := Campaign{
		TemplateBody: "<html>{{ template \"content\" . }}</html>",
		Body:         "Body",
		AltBody:      null.StringFrom("Alt {{ .Name | invalidFunc }}"),
	}
	if err := badAlt.CompileTemplate(fMap); err == nil {
		t.Fatalf("expected error compiling bad alt body")
	}

	// Bad Header
	badHdr := Campaign{
		TemplateBody: "<html>{{ template \"content\" . }}</html>",
		Body:         "Body",
		Headers: Headers{
			map[string]string{"X-Test": "Bad {{ .Name | invalidFunc }}"},
		},
	}
	if err := badHdr.CompileTemplate(fMap); err == nil {
		t.Fatalf("expected error compiling bad header template")
	}
}
