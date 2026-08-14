package models

import (
	"html/template"
	"testing"
)

func TestTemplate_Compile_PromptType(t *testing.T) {
	tpl := Template{
		Name: "Prompt Template",
		Type: TemplateTypePrompt,
		Body: "You are an AI assistant. Help user {{ .Name }}.",
	}

	fMap := template.FuncMap{}
	if err := tpl.Compile(fMap); err != nil {
		t.Fatalf("unexpected error compiling prompt template: %v", err)
	}

	if tpl.SubjectTpl == nil {
		t.Fatalf("expected SubjectTpl to be compiled for prompt template body")
	}
}

func TestTemplate_Compile_StandardType(t *testing.T) {
	tpl := Template{
		Name:    "HTML Template",
		Type:    TemplateTypeCampaign,
		Subject: "Hello {{ .Name }}",
		Body:    "<html><body>{{ template \"content\" . }}</body></html>",
	}

	fMap := template.FuncMap{}
	if err := tpl.Compile(fMap); err != nil {
		t.Fatalf("unexpected error compiling standard template: %v", err)
	}

	if tpl.Tpl == nil {
		t.Fatalf("expected Tpl to be compiled")
	}
	if tpl.SubjectTpl == nil {
		t.Fatalf("expected SubjectTpl to be compiled")
	}
}
