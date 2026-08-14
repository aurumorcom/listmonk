package subimporter

import (
	"testing"

	"github.com/knadh/listmonk/internal/i18n"
	"github.com/knadh/listmonk/models"
	null "gopkg.in/volatiletech/null.v6"
)

func TestValidateFields_Phone(t *testing.T) {
	langJSON := []byte(`{
		"_.code": "en",
		"_.name": "English",
		"subscribers.invalidEmail": "Invalid email.",
		"subscribers.invalidPhone": "Invalid phone number."
	}`)
	i18nInst, err := i18n.New(langJSON)
	if err != nil {
		t.Fatalf("failed to initialize i18n: %v", err)
	}
	im := &Importer{
		i18n: i18nInst,
	}

	tests := []struct {
		name        string
		req         SubReq
		expectErr   bool
		expectPhone string
	}{
		{
			name: "valid phone normalized to E.164",
			req: SubReq{
				Subscriber: models.Subscriber{
					Email: "alice@example.com",
					Phone: null.StringFrom("+1 (415) 555-2671"),
				},
			},
			expectErr:   false,
			expectPhone: "+14155552671",
		},
		{
			name: "valid phone without plus normalized to E.164",
			req: SubReq{
				Subscriber: models.Subscriber{
					Email: "bob@example.com",
					Phone: null.StringFrom("442079460958"),
				},
			},
			expectErr:   false,
			expectPhone: "+442079460958",
		},
		{
			name: "phone in attribs populated and normalized",
			req: SubReq{
				Subscriber: models.Subscriber{
					Email:   "carol@example.com",
					Attribs: models.JSON{"phone": "+91 94723 80340"},
				},
			},
			expectErr:   false,
			expectPhone: "+919472380340",
		},
		{
			name: "empty phone remains empty without error",
			req: SubReq{
				Subscriber: models.Subscriber{
					Email: "dave@example.com",
				},
			},
			expectErr:   false,
			expectPhone: "",
		},
		{
			name: "invalid phone returns error",
			req: SubReq{
				Subscriber: models.Subscriber{
					Email: "eve@example.com",
					Phone: null.StringFrom("0712345678"),
				},
			},
			expectErr: true,
		},
		{
			name: "invalid phone in attribs returns error",
			req: SubReq{
				Subscriber: models.Subscriber{
					Email:   "frank@example.com",
					Attribs: models.JSON{"phone": "invalid-num"},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := im.ValidateFields(tt.req)
			if (err != nil) != tt.expectErr {
				t.Fatalf("ValidateFields() error = %v, expectErr %v", err, tt.expectErr)
			}
			if !tt.expectErr {
				if tt.expectPhone == "" {
					if res.Phone.Valid && res.Phone.String != "" {
						t.Errorf("expected empty phone, got %s", res.Phone.String)
					}
				} else {
					if !res.Phone.Valid || res.Phone.String != tt.expectPhone {
						t.Errorf("expected phone %s, got %v", tt.expectPhone, res.Phone)
					}
				}
			}
		})
	}
}
