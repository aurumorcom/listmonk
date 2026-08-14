package utils

import (
	"crypto/rand"
	"errors"
	"net/mail"
	"net/url"
	"path"
	"strings"

	"github.com/nyaruka/phonenumbers"
)

// ErrInvalidEmail is returned by SanitizeEmail for malformed input.
var ErrInvalidEmail = errors.New("invalid e-mail address")

// ErrInvalidPhone is returned by SanitizePhone for malformed input.
var ErrInvalidPhone = errors.New("invalid phone number")

// ValidateEmail reports whether s is a correctly formed bare e-mail address
// (no display name component).
func ValidateEmail(s string) bool {
	_, err := SanitizeEmail(s)
	return err == nil
}

// ValidatePhone reports whether s is a valid international phone number.
func ValidatePhone(s string) bool {
	_, err := SanitizePhone(s)
	return err == nil
}

// SanitizePhone validates and sanitizes a phone number using Google's libphonenumber,
// returning the canonical E.164 formatted string (e.g. +14155552671).
// An empty string is considered valid and returns ("", nil) since phone is optional.
func SanitizePhone(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}

	// Reject any alphabetic characters (vanity words, letters)
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return "", ErrInvalidPhone
		}
	}

	// Prepare parse string: normalize leading 00 to + or prepend + if missing
	target := s
	if strings.HasPrefix(target, "00") {
		target = "+" + target[2:]
	} else if !strings.HasPrefix(target, "+") {
		target = "+" + target
	}

	num, err := phonenumbers.Parse(target, "")
	if err != nil {
		// Fallback retry with raw input if preprocessed failed
		num, err = phonenumbers.Parse(s, "")
		if err != nil {
			return "", ErrInvalidPhone
		}
	}

	if !phonenumbers.IsValidNumber(num) {
		return "", ErrInvalidPhone
	}

	return phonenumbers.Format(num, phonenumbers.E164), nil
}

// SanitizeEmail trims, lowercases, and validates s as a bare e-mail address
// (no display name) and returns the canonical form. Returns ErrInvalidEmail
// for anything `mail.ParseAddress` rejects or for input with a display name.
func SanitizeEmail(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	em, err := mail.ParseAddress(s)
	if err != nil || em.Address != s {
		return "", ErrInvalidEmail
	}
	return em.Address, nil
}

// ParseEmailAddress extracts the lowercased bare address from an RFC 5322
// "From"-style header value, accepting both bare addresses ("a@b.com") and
// the display-name form ("Name <a@b.com>"). Returns "" if unparseable.
func ParseEmailAddress(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	em, err := mail.ParseAddress(s)
	if err != nil {
		return ""
	}
	return strings.ToLower(em.Address)
}

// GenerateRandomString generates a cryptographically random, alphanumeric string of length n.
func GenerateRandomString(n int) (string, error) {
	const dictionary = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	var bytes = make([]byte, n)

	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	for k, v := range bytes {
		bytes[k] = dictionary[v%byte(len(dictionary))]
	}

	return string(bytes), nil
}

// SanitizeURI takes a URL or URI, removes the domain from it, returns only the URI.
// This is used for cleaning "next" redirect URLs/URIs to prevent open redirects.
func SanitizeURI(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return "/"
	}

	p, err := url.Parse(u)
	if err != nil || strings.Contains(p.Path, "..") {
		return "/"
	}

	return path.Clean(p.Path)
}
