package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/internal/manager"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
	null "gopkg.in/volatiletech/null.v6"
)

const (
	// tplTag is the template tag that should be present in a template
	// as the placeholder for campaign bodies.
	tplTag = `{{ template "content" . }}`

	dummyTpl = `
		<p>Hi there</p>
		<p>Lorem ipsum dolor sit amet, consectetur adipiscing elit. Duis et elit ac elit sollicitudin condimentum non a magna. Sed tempor mauris in facilisis vehicula. Aenean nisl urna, accumsan ac tincidunt vitae, interdum cursus massa. Interdum et malesuada fames ac ante ipsum primis in faucibus. Aliquam varius turpis et turpis lacinia placerat. Aenean id ligula a orci lacinia blandit at eu felis. Phasellus vel lobortis lacus. Suspendisse leo elit, luctus sed erat ut, venenatis fermentum ipsum. Donec bibendum neque quis.</p>

		<h3>Sub heading</h3>
		<p>Nam luctus dui non placerat mattis. Morbi non accumsan orci, vel interdum urna. Duis faucibus id nunc ut euismod. Curabitur et eros id erat feugiat fringilla in eget neque. Aliquam accumsan cursus eros sed faucibus.</p>

		<p>Here is a link to <a href="https://listmonk.app" target="_blank">listmonk</a>.</p>`
)

var (
	regexpTplTag = regexp.MustCompile(`{{(\s+)?template\s+?"content"(\s+)?\.(\s+)?}}`)
)

// GetTemplate handles the retrieval of a template
func (a *App) GetTemplate(c echo.Context) error {
	// If no_body is true, blank out the body of the template from the response.
	noBody, _ := strconv.ParseBool(c.QueryParam("no_body"))

	// Get the template from the DB.
	id := getID(c)
	out, err := a.core.GetTemplate(id, noBody)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{out})
}

// GetTemplates handles retrieval of templates.
func (a *App) GetTemplates(c echo.Context) error {
	// If no_body is true, blank out the body of the template from the response.
	noBody, _ := strconv.ParseBool(c.QueryParam("no_body"))

	// Fetch templates from the DB.
	out, err := a.core.GetTemplates("", noBody)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{out})
}

// getSubscriberForPreview resolves the preview contact for template/campaign/sequence rendering.
// If subID > 0, it attempts to fetch that specific subscriber.
// If no subscriber exists, it falls back to dummySubscriber.
func (a *App) getSubscriberForPreview(subID int) models.Subscriber {
	if subID > 0 {
		if sub, err := a.core.GetSubscriber(subID, "", ""); err == nil {
			return sub
		}
	}
	return dummySubscriber
}

// resolveTestPreviewSubscriber resolves the subscriber context for test message template rendering.
// 1. If explicit subID > 0, fetches that subscriber.
// 2. If logged in user email exists and matches an existing subscriber in DB, uses that subscriber.
// 3. If logged in user phone exists and matches an existing subscriber in DB, uses that subscriber.
// 4. If logged in user has a name/email/phone, builds a subscriber context from the logged-in user profile.
// 5. Fallback to dummySubscriber.
func (a *App) resolveTestPreviewSubscriber(subID int, user auth.User) models.Subscriber {
	if a.core != nil {
		if subID > 0 {
			if sub, err := a.core.GetSubscriber(subID, "", ""); err == nil && sub.ID > 0 {
				return sub
			}
		}

		if user.Email.Valid && strings.TrimSpace(user.Email.String) != "" {
			if sub, err := a.core.GetSubscriber(0, "", strings.TrimSpace(user.Email.String)); err == nil && sub.ID > 0 {
				return sub
			}
		}

		if user.Phone.Valid && strings.TrimSpace(user.Phone.String) != "" {
			if sub, err := a.core.GetSubscriberByPhone(strings.TrimSpace(user.Phone.String)); err == nil && sub.ID > 0 {
				return sub
			}
		}
	}

	if user.Name != "" || (user.Email.Valid && user.Email.String != "") {
		name := user.Name
		if name == "" {
			name = user.Username
		}
		return models.Subscriber{
			Name:    name,
			Email:   user.Email.String,
			Phone:   user.Phone,
			Status:  models.SubscriberStatusEnabled,
			Attribs: models.JSON{"first_name": name, "name": name},
		}
	}

	return dummySubscriber
}

// PreviewTemplate renders the HTML preview of a template in the DB.
func (a *App) PreviewTemplate(c echo.Context) error {
	// Fetch one template from the DB.
	id := getID(c)
	tpl, err := a.core.GetTemplate(id, false)
	if err != nil {
		return err
	}

	subID, _ := strconv.Atoi(c.FormValue("subscriber_id"))
	if subID == 0 {
		subID, _ = strconv.Atoi(c.QueryParam("subscriber_id"))
	}
	sub := a.getSubscriberForPreview(subID)

	// Render the template.
	out, err := a.previewTemplate(tpl, sub)
	if err != nil {
		return err
	}

	return c.HTML(http.StatusOK, string(out))
}

// PreviewTemplateBody renders the HTML preview of a template given its type and body.
func (a *App) PreviewTemplateBody(c echo.Context) error {
	var parentID null.Int
	if pID, err := strconv.Atoi(c.FormValue("parent_template_id")); err == nil && pID > 0 {
		parentID = null.IntFrom(pID)
	}
	tpl := models.Template{
		Type:             c.FormValue("template_type"),
		ParentTemplateID: parentID,
		Body:             c.FormValue("body"),
	}

	// Body is posted with the request.
	if tpl.Type == "" {
		tpl.Type = models.TemplateTypeCampaign
	}

	if tpl.Type == models.TemplateTypeCampaign && !regexpTplTag.MatchString(tpl.Body) {
		return echo.NewHTTPError(http.StatusBadRequest,
			a.i18n.Ts("templates.placeholderHelp", "placeholder", tplTag))
	}

	subID, _ := strconv.Atoi(c.FormValue("subscriber_id"))
	if subID == 0 {
		subID, _ = strconv.Atoi(c.QueryParam("subscriber_id"))
	}
	sub := a.getSubscriberForPreview(subID)

	// Render the template.
	out, err := a.previewTemplate(tpl, sub)
	if err != nil {
		return err
	}

	return c.HTML(http.StatusOK, string(out))
}

// CreateTemplate handles template creation.
func (a *App) CreateTemplate(c echo.Context) error {
	var o models.Template
	if err := c.Bind(&o); err != nil {
		return err
	}
	if err := a.validateTemplate(o); err != nil {
		return err
	}

	// Subject is only relevant for fixed tx templates. For campaigns,
	// the subject changes per campaign and is on models.Campaign.
	var funcs template.FuncMap
	if o.Type == models.TemplateTypeCampaign || o.Type == models.TemplateTypeCampaignVisual {
		o.Subject = ""
		funcs = a.manager.TemplateFuncs(nil)
	} else {
		funcs = a.manager.GenericTemplateFuncs()
	}

	// Compile the template and validate.
	if err := o.Compile(funcs); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Create the template the in the DB.
	out, err := a.core.CreateTemplate(o.Name, o.Type, o.Subject, []byte(o.Body), o.BodySource, o.ParentTemplateID)
	if err != nil {
		return err
	}

	// If it's a transactional template, cache it in the manager
	// to be used for arbitrary incoming tx message pushes.
	if o.Type == models.TemplateTypeTx {
		a.manager.CacheTpl(out.ID, &o)
	}

	return c.JSON(http.StatusOK, okResp{out})
}

// UpdateTemplate handles template modification.
func (a *App) UpdateTemplate(c echo.Context) error {
	var o models.Template
	if err := c.Bind(&o); err != nil {
		return err
	}
	if err := a.validateTemplate(o); err != nil {
		return err
	}

	// Subject is only relevant for fixed tx templates. For campaigns,
	// the subject changes per campaign and is on models.Campaign.
	var funcs template.FuncMap
	if o.Type == models.TemplateTypeCampaign || o.Type == models.TemplateTypeCampaignVisual {
		o.Subject = ""
		funcs = a.manager.TemplateFuncs(nil)
	} else {
		funcs = a.manager.GenericTemplateFuncs()
	}

	// Compile the template and validate.
	if err := o.Compile(funcs); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Update the template in the DB.
	id := getID(c)
	out, err := a.core.UpdateTemplate(id, o.Name, o.Subject, []byte(o.Body), o.BodySource, o.ParentTemplateID)
	if err != nil {
		return err
	}

	// If it's a transactional template, cache it.
	if out.Type == models.TemplateTypeTx {
		a.manager.CacheTpl(out.ID, &o)
	}

	return c.JSON(http.StatusOK, okResp{out})

}

// TemplateSetDefault handles template modification.
func (a *App) TemplateSetDefault(c echo.Context) error {
	// Update the template in the DB.
	id := getID(c)
	if err := a.core.SetDefaultTemplate(id); err != nil {
		return err
	}

	return a.GetTemplates(c)
}

// DeleteTemplate handles template deletion.
func (a *App) DeleteTemplate(c echo.Context) error {
	// Delete the template from the DB.
	id := getID(c)
	if err := a.core.DeleteTemplate(id); err != nil {
		return err
	}

	// Delete cached in-memory template.
	a.manager.DeleteTpl(id)

	return c.JSON(http.StatusOK, okResp{true})
}

// compileTemplate validates template fields.
func (a *App) validateTemplate(o models.Template) error {
	if !strHasLen(o.Name, 1, stdInputMaxLen) {
		return errors.New(a.i18n.T("campaigns.fieldInvalidName"))
	}

	if o.Type == models.TemplateTypeCampaign && !regexpTplTag.MatchString(o.Body) {
		return echo.NewHTTPError(http.StatusBadRequest,
			a.i18n.Ts("templates.placeholderHelp", "placeholder", tplTag))
	}

	if o.Type == models.TemplateTypeTx && strings.TrimSpace(o.Subject) == "" {
		return echo.NewHTTPError(http.StatusBadRequest,
			a.i18n.Ts("globals.messages.missingFields", "name", "subject"))
	}

	return nil
}

// previewTemplate renders the HTML preview of a template with contact context and signature precedence.
func (a *App) previewTemplate(tpl models.Template, sub models.Subscriber) ([]byte, error) {
	var out []byte
	if tpl.Type == models.TemplateTypePrompt {
		if err := tpl.Compile(a.manager.GenericTemplateFuncs()); err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		scope := manager.ExtractTemplateScope(sub)
		sysPromptStr := tpl.Body
		if tpl.SubjectTpl != nil {
			var sb bytes.Buffer
			if err := tpl.SubjectTpl.Execute(&sb, scope); err == nil {
				sysPromptStr = sb.String()
			}
		}

		userPromptStr := "Generate a preview message for " + sub.Name
		var contentToWrap string

		if bc := a.manager.BifrostClient(); bc != nil {
			ctx, cancel := bc.TimeoutContext()
			aiBody, err := bc.GeneratePromptWithFormat(ctx, sysPromptStr, userPromptStr, manager.EmailResponseFormat())
			cancel()
			if err == nil && aiBody != "" {
				cleanBody := manager.CleanJSONResponse(aiBody)
				var emailOut manager.EmailStructuredOutput
				if err := json.Unmarshal([]byte(cleanBody), &emailOut); err == nil && emailOut.Content != "" {
					var globalSig string
					if st, err := a.core.GetSettings(); err == nil {
						globalSig = st.AppGlobalSignature
					}
					sig := manager.ResolveSignatureAdvanced(manager.SignatureOpts{
						Subscriber: sub,
						GlobalSig:  globalSig,
					})
					contentToWrap = manager.FormatPlainTextWithSignature(emailOut.Content, sig)
				} else {
					contentToWrap = aiBody
				}
			}
		}

		if contentToWrap == "" {
			var globalSig string
			if st, err := a.core.GetSettings(); err == nil {
				globalSig = st.AppGlobalSignature
			}
			sig := manager.ResolveSignatureAdvanced(manager.SignatureOpts{
				Subscriber: sub,
				GlobalSig:  globalSig,
			})
			renderedBody := fmt.Sprintf("System Prompt:\n%s\n\nSample User Task:\n%s", sysPromptStr, userPromptStr)
			contentToWrap = manager.FormatPlainTextWithSignature(renderedBody, sig)
		}

		// If ParentTemplateID is set, wrap content in parent layout
		if tpl.ParentTemplateID.Valid && tpl.ParentTemplateID.Int > 0 {
			if parentTpl, err := a.core.GetTemplate(int(tpl.ParentTemplateID.Int), false); err == nil {
				camp := models.Campaign{
					UUID:         dummyUUID,
					Name:         a.i18n.T("templates.dummyName"),
					Subject:      a.i18n.T("templates.dummySubject"),
					FromEmail:    "dummy-campaign@listmonk.app",
					TemplateBody: parentTpl.Body,
					Body:         contentToWrap,
				}
				if err := camp.CompileTemplate(a.manager.TemplateFuncs(&camp)); err == nil {
					if msg, err := a.manager.NewCampaignMessage(&camp, sub); err == nil {
						return msg.Body(), nil
					}
				}
			}
		}

		return []byte(fmt.Sprintf("<!DOCTYPE html><html><body style='margin: 0; padding: 1.5em; background-color: #f9fafb;'><div style='font-family: -apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, Helvetica, Arial, sans-serif; font-size: 14px; line-height: 1.6; color: #111827; white-space: pre-wrap; background: #ffffff; padding: 1.5em; border-radius: 6px; border: 1px solid #e5e7eb;'>%s</div></body></html>", html.EscapeString(contentToWrap))), nil
	} else if tpl.Type == models.TemplateTypeCampaign || tpl.Type == models.TemplateTypeCampaignVisual {
		camp := models.Campaign{
			UUID:         dummyUUID,
			Name:         a.i18n.T("templates.dummyName"),
			Subject:      a.i18n.T("templates.dummySubject"),
			FromEmail:    "dummy-campaign@listmonk.app",
			TemplateBody: tpl.Body,
			Body:         dummyTpl,
		}

		if err := camp.CompileTemplate(a.manager.TemplateFuncs(&camp)); err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest,
				a.i18n.Ts("templates.errorCompiling", "error", err.Error()))
		}

		// Render the message body.
		msg, err := a.manager.NewCampaignMessage(&camp, sub)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest,
				a.i18n.Ts("templates.errorRendering", "error", err.Error()))
		}
		out = msg.Body()
	} else {
		// Compile transactional template.
		if err := tpl.Compile(a.manager.GenericTemplateFuncs()); err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		m := models.TxMessage{
			Subject: tpl.Subject,
		}

		// Render the message.
		if err := m.Render(sub, &tpl, a.manager.GenericTemplateFuncs()); err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		out = m.Body
	}

	return out, nil
}
