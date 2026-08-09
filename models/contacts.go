package models

import "strings"

// Contact is a domain-driven type alias of Subscriber representing an individual recipient
// across multi-channel outreach, cadences, and CRM contexts.
type Contact = Subscriber

// Contacts represents a slice of Contact.
type Contacts = Subscribers

// ContactSummary provides a lightweight representation of a contact for outreach lists.
type ContactSummary struct {
	ID    int    `json:"id"`
	UUID  string `json:"uuid"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone,omitempty"`
}

// PrimaryIdentifier returns email if non-empty, otherwise returns phone if valid, or empty string.
func (c Contact) PrimaryIdentifier() string {
	if strings.TrimSpace(c.Email) != "" {
		return strings.TrimSpace(c.Email)
	}
	if c.Phone.Valid && strings.TrimSpace(c.Phone.String) != "" {
		return strings.TrimSpace(c.Phone.String)
	}
	return ""
}

// ChannelAddress returns recipient address string based on target messenger.
func (c Contact) ChannelAddress(messenger string) string {
	switch strings.ToLower(strings.TrimSpace(messenger)) {
	case "whatsapp", "waha", "sms":
		if c.Phone.Valid && strings.TrimSpace(c.Phone.String) != "" {
			return strings.TrimSpace(c.Phone.String)
		}
	}
	return c.Email
}

// Company extracts company name from custom JSON attributes if available.
func (c Contact) Company() string {
	if c.Attribs == nil {
		return ""
	}
	if v, ok := c.Attribs["company"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}
