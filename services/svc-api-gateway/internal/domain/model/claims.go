package model

import "time"

type PasetoClaims struct {
	Subject    string    `json:"sub"`
	Issuer     string    `json:"iss"`
	Audience   string    `json:"aud"`
	Expiration time.Time `json:"exp"`
	IssuedAt   time.Time `json:"iat"`
	NotBefore  time.Time `json:"nbf"`
	TokenID    string    `json:"jti"`
	Roles      []string  `json:"roles,omitempty"`
}

func (c *PasetoClaims) IsExpired() bool {
	return time.Now().After(c.Expiration)
}

func (c *PasetoClaims) IsNotYetValid() bool {
	return time.Now().Before(c.NotBefore)
}

func (c *PasetoClaims) HasRole(role string) bool {
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}

	return false
}
